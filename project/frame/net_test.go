package frame

import (
	"google.golang.org/protobuf/proto"
	"net"
	"server/protocol/generate/pb"
	"sync"
	"testing"
	"time"
)

func Test_TcpMsgQue(t *testing.T) {
	// 动态获取可用端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("动态端口获取失败:", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close() // 释放临时监听器

	// 使用通道同步服务器启动状态
	serverReady := make(chan struct{})
	var serverErr error

	msgHandler := NewMsgHandler()
	//msgHandler.AddHandler(1, func(body []byte) bool {
	//	t.Log("收到消息:", body)
	//	return true
	//})
	msgHandler.AddHandlers(CSHandlerMap)
	CSHandlerObj.HandlerLogin = func(req *pb.LoginC2S) error {
		LogInfo("HandlerLogin: %v", req)
		return nil
	}
	CSHandlerObj.HandlerCommon = func(req *pb.CommonC2S) error {
		LogInfo("HandlerCommon: %v", req)
		return nil
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	// 启动服务器
	go func() {
		defer wg.Done()
		serverErr = TcpListen(addr, msgHandler)
		close(serverReady) // 通知服务器已启动（无论成功与否）
	}()

	// 等待服务器启动完成
	select {
	case <-serverReady:
		if serverErr != nil {
			t.Fatal("服务器启动失败:", serverErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("服务器启动超时")
	}

	// 启动客户端
	go func() {
		defer wg.Done()
		mq := newTcpConnect("tcp", addr, msgHandler)
		mq.Reconnect(0) // 立即连接

		// 连接状态检查
		for i := 0; i < 10; i++ { // 最多重试 10 次
			if mq.conn != nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
		if mq.conn == nil {
			t.Error("连接未建立")
		}

		// 发送消息
		msg := &Message{
			Head: &MessageHead{ProtoId: 1, Length: 3},
			//Body: []byte{0x01, 0x02, 0x03},
		}

		msg.Body, _ = proto.Marshal(&pb.LoginC2S{RoleId: 123})
		msg.Head.Length = uint32(len(msg.Body))

		if msg.Bytes() == nil {
			t.Error("消息编码失败")
		}
		if !mq.Send(msg) {
			t.Error("消息发送失败")
		}

		msg = &Message{
			Head: &MessageHead{ProtoId: 2, Length: 3},
			//Body: []byte{0x02, 0x03, 0x04},
		}

		msg.Body, _ = proto.Marshal(&pb.CommonC2S{RoleId: 123})
		msg.Head.Length = uint32(len(msg.Body))

		if msg.Bytes() == nil {
			t.Error("消息编码失败")
		}
		if !mq.Send(msg) {
			t.Error("消息发送失败")
		}

		// 等待消息处理
		time.Sleep(1 * time.Second)
	}()

	wg.Wait()
}
