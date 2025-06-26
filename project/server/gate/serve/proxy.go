package serve

import (
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"net/http"
	"server/frame"
	"server/protocol/generate/pb"
	"sync/atomic"
)

type gateProxy struct {
	clientConn *websocket.Conn
	cRead      chan []byte
	cWrite     chan []byte
	id         uint32
}

func (r *gateProxy) ReadMsg() {
	frame.Gogo(func() {
		defer func() {
		}()

		for {
			select {
			case data := <-r.cRead:
				if data == nil {
					return
				}
				head := &frame.MessageHead{}
				if err := head.Decode(data[:frame.MsgHeadSize]); err != nil {
					frame.LogError("gateProxy read head err:%v", err)
					return
				}
				// 转发到逻辑服
				rpc := frame.GetProtoService(int32(pb.ProtocolId_Login))
				if rpc == nil {
					frame.LogError("get proto service fail")
					continue
				}
				rpc.Send(frame.NewBytesMsg(head.ProtoId, data[frame.MsgHeadSize:]))
			}
		}
	})
}

func (r *gateProxy) WriteMsg() {
	frame.Gogo(func() {
		defer func() {
			if r.clientConn != nil {
				_ = r.clientConn.Close()
			}
		}()

		for {
			select {
			case data := <-r.cWrite:
				if data == nil {
					return
				}
				// 回复给玩家客户端
				err := r.clientConn.WriteMessage(websocket.BinaryMessage, data)
				if err != nil {
					frame.LogError("gateProxy write err:%v", err)
					return
				}
			}
		}
	})
}

var (
	proxyId    uint32 = 0
	wsUpgrader        = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func StartProxy(writer http.ResponseWriter, request *http.Request) {
	conn, upgradeErr := wsUpgrader.Upgrade(writer, request, nil)
	if upgradeErr != nil {
		frame.LogError("upgrade err:%v", upgradeErr)
		return
	}

	frame.Gogo(func() {
		// 构造代理对象
		proxy := &gateProxy{
			clientConn: conn,
			cRead:      make(chan []byte, 64),
			cWrite:     make(chan []byte, 64),
			id:         atomic.AddUint32(&proxyId, 1),
		}

		// 首条消息需要为登录
		_, firstData, readErr := proxy.clientConn.ReadMessage()
		if readErr != nil {
			frame.LogError("StartProxy read first message err:%v", readErr)
			return
		}
		loginC2S := &pb.LoginC2S{}
		if decodeErr := proto.Unmarshal(firstData, loginC2S); decodeErr != nil {
			frame.LogError("StartProxy unmarshal first message err:%v", decodeErr)
			return
		}

		// 转发消息到逻辑服
		rpc := frame.GetProtoService(int32(pb.ProtocolId_Login))
		if rpc == nil {
			frame.LogError("get proto service fail")
			return
		}
		rpc.Send(frame.NewProtoMsg(pb.ProtocolId_Login, loginC2S))

		// 收发消息
		proxy.ReadMsg()
		proxy.WriteMsg()
		defer func() {
			close(proxy.cRead)
			close(proxy.cWrite)
		}()
		for {
			_, data, err := proxy.clientConn.ReadMessage()
			if err != nil {
				frame.LogError("StartProxy read normal message err:%v", err)
				return
			}
			proxy.cRead <- data
		}
	})
}
