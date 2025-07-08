package main

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"server/protocol/generate/pb"
	"time"
)

type GameServer struct {
	pb.UnimplementedMessageServiceServer
}

// HandleMessage 实现接口MessageService
func (g *GameServer) HandleMessage(ctx context.Context, req *pb.ClientRequest) (*pb.ServerResponse, error) {
	log.Printf("处理消息: 会话=%s, ID=%d, 长度=%d",
		req.SessionId, req.MsgId, len(req.Payload))

	// 模拟消息处理
	time.Sleep(3 * time.Millisecond)

	// 创建响应
	response := &pb.ServerResponse{
		SessionId: req.SessionId,
		MsgId:     req.MsgId,
		Payload:   []byte("处理结果: 操作成功"),
	}

	return response, nil
}

// 注册到Gate服务器
func registerToGate(gateAddr string) {
	// 创建连接
	conn, err := grpc.NewClient(
		gateAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("连接Gate服务器失败: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// 创建客户端
	client := pb.NewRegistryServiceClient(conn)

	// 准备服务信息
	info := &pb.ServerInfo{
		Id:          1,
		Type:        "game",
		Address:     "localhost:60051",                // 本服务地址
		MsgHandlers: []int32{100, 101, 102, 200, 201}, // 能处理的消息ID
	}

	// 注册服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.HelloShake(ctx, info)
	if err != nil {
		log.Fatalf("注册失败: %v", err)
	}

	log.Println("成功注册到Gate服务器")
}

func main() {
	// 启动gRPC服务
	lis, err := net.Listen("tcp", ":60051")
	if err != nil {
		log.Fatalf("监听失败: %v", err)
	}

	grpcServer := grpc.NewServer()
	gameServer := &GameServer{}
	pb.RegisterMessageServiceServer(grpcServer, gameServer)

	go func() {
		log.Println("Game服务启动，监听 :60051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC服务启动失败: %v", err)
		}
	}()

	// 注册到Gate服务器
	registerToGate("localhost:50051")

	// 保持运行
	select {}
}
