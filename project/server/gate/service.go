package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"net"
	"net/http"
	"server/protocol/generate/pb"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Session 客户端会话管理
type Session struct {
	Conn      *websocket.Conn
	SessionID string
	CreatedAt time.Time
}

// ServiceInfo 服务注册信息
type ServiceInfo struct {
	ID          int32
	Type        string
	Address     string
	MsgHandlers []int32 // 能处理的消息ID
	Client      pb.MessageServiceClient
	LastActive  time.Time
}

// GateServer Gate服务器结构
type GateServer struct {
	pb.UnimplementedRegistryServiceServer

	sessions     sync.Map                 // 客户端会话
	services     map[int32]*ServiceInfo   // 注册的服务 (key:服务ID)
	serviceByMsg map[int32][]*ServiceInfo // 服务的映射 (key:消息ID)
	serviceMutex sync.RWMutex
	grpcServer   *grpc.Server
	lbCounters   map[int32]int // 负载均衡计数器 (key:消息ID)
}

// HelloShake 服务注册握手
func (g *GateServer) HelloShake(ctx context.Context, info *pb.ServerInfo) (*emptypb.Empty, error) {
	g.serviceMutex.Lock()
	defer g.serviceMutex.Unlock()

	log.Printf("服务注册: ID=%d, 类型=%s, 地址=%s", info.Id, info.Type, info.Address)

	// 创建服务客户端
	conn, err := grpc.NewClient(
		info.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		log.Printf("创建服务客户端失败: %v", err)
		return nil, err
	}

	serviceClient := pb.NewMessageServiceClient(conn)

	// 创建服务信息
	service := &ServiceInfo{
		ID:          info.Id,
		Type:        info.Type,
		Address:     info.Address,
		MsgHandlers: info.MsgHandlers,
		Client:      serviceClient,
		LastActive:  time.Now(),
	}

	// 更新服务注册
	g.services[info.Id] = service

	// 更新消息ID到服务的映射
	for _, msgID := range info.MsgHandlers {
		g.serviceByMsg[msgID] = append(g.serviceByMsg[msgID], service)
		log.Printf("消息ID %d 可由服务 %d (%s) 处理", msgID, info.Id, info.Type)
	}

	log.Printf("已注册服务: %s, 处理消息: %v", info.Type, info.MsgHandlers)
	return &emptypb.Empty{}, nil
}

// 处理WebSocket连接
func (g *GateServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	// 创建会话
	sessionID := func() string {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		return hex.EncodeToString(b)
	}()
	session := &Session{
		Conn:      conn,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}
	g.sessions.Store(sessionID, session)
	defer g.sessions.Delete(sessionID)

	log.Printf("新客户端连接: %s", sessionID)

	// 监听客户端消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("读取消息失败: %v", err)
			break
		}

		// 解析消息ID (前4字节)
		if len(message) < 4 {
			log.Printf("无效消息长度")
			continue
		}
		msgID := int32(message[0]) | int32(message[1])<<8 | int32(message[2])<<16 | int32(message[3])<<24
		payload := message[4:]

		log.Printf("收到消息: 会话=%s, ID=%d, 长度=%d", sessionID, msgID, len(payload))

		// 路由消息到合适的服务
		go g.routeMessage(sessionID, msgID, payload)
	}
}

// 路由消息到处理服务
func (g *GateServer) routeMessage(sessionID string, msgID int32, payload []byte) {
	g.serviceMutex.RLock()
	defer g.serviceMutex.RUnlock()

	// 查找能处理该消息ID的服务
	services, exists := g.serviceByMsg[msgID]
	if !exists || len(services) == 0 {
		log.Printf("没有服务能处理消息ID: %d", msgID)
		return
	}

	// 使用负载均衡策略选择服务
	service := g.selectService(msgID, services)
	if service == nil {
		log.Printf("没有可用的服务处理消息ID: %d", msgID)
		return
	}

	// 创建请求
	req := &pb.ClientRequest{
		SessionId: sessionID,
		MsgId:     msgID,
		Payload:   payload,
	}

	// 调用服务
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := service.Client.HandleMessage(ctx, req)
	if err != nil {
		log.Printf("处理消息失败: %v", err)
		return
	}

	// 发送响应回客户端
	g.sendToClient(resp)
}

// 负载均衡选择服务 (轮询策略)
func (g *GateServer) selectService(msgID int32, services []*ServiceInfo) *ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	// 初始化计数器
	if _, exists := g.lbCounters[msgID]; !exists {
		g.lbCounters[msgID] = 0
	}

	// 轮询选择
	index := g.lbCounters[msgID] % len(services)
	g.lbCounters[msgID] = (g.lbCounters[msgID] + 1) % 1000000 // 避免溢出

	return services[index]
}

// 发送响应到客户端
func (g *GateServer) sendToClient(resp *pb.ServerResponse) {
	if session, ok := g.sessions.Load(resp.SessionId); ok {
		// 构建响应消息 (4字节ID + payload)
		msg := make([]byte, 4+len(resp.Payload))
		msg[0] = byte(resp.MsgId)
		msg[1] = byte(resp.MsgId >> 8)
		msg[2] = byte(resp.MsgId >> 16)
		msg[3] = byte(resp.MsgId >> 24)
		copy(msg[4:], resp.Payload)

		if err := session.(*Session).Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			log.Printf("发送响应失败: %v", err)
		}
	} else {
		log.Printf("会话不存在: %s", resp.SessionId)
	}
}

// 健康检查 - 定期检查服务可用性
func (g *GateServer) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		g.serviceMutex.RLock()
		servicesToCheck := map[int32][]*ServiceInfo{}
		for msgID, services := range g.serviceByMsg {
			servicesToCheck[msgID] = services
		}
		g.serviceMutex.RUnlock()

		for msgID, services := range servicesToCheck {
			var activeServices []*ServiceInfo
			for _, service := range services {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_, err := service.Client.HandleMessage(ctx, &pb.ClientRequest{MsgId: -1})
				cancel()

				if err == nil {
					activeServices = append(activeServices, service)
					service.LastActive = time.Now()
				} else {
					log.Printf("服务不可用: ID=%d, 类型=%s, 地址=%s", service.ID, service.Type, service.Address)
					delete(g.services, service.ID)
				}
			}
			g.serviceMutex.Lock()
			g.serviceByMsg[msgID] = activeServices
			g.serviceMutex.Unlock()
		}
	}
}

// Start Gate启动服务
func (g *GateServer) Start() {
	// 启动健康检查
	go g.healthCheck()

	// 启动gRPC服务（供逻辑服务器注册）
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("监听失败: %v", err)
	}

	g.grpcServer = grpc.NewServer()
	pb.RegisterRegistryServiceServer(g.grpcServer, g)

	go func() {
		log.Println("Gate注册服务启动，监听 :50051")
		if err := g.grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC服务启动失败: %v", err)
		}
	}()

	// 启动WebSocket服务
	http.HandleFunc("/ws", g.handleWebSocket)
	log.Println("WebSocket服务启动，监听 :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("WebSocket服务器启动失败: %v", err)
	}
}

func main() {
	gate := &GateServer{
		services:     make(map[int32]*ServiceInfo),
		serviceByMsg: make(map[int32][]*ServiceInfo),
		lbCounters:   make(map[int32]int),
	}
	gate.Start()
}
