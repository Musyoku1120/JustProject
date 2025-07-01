package frame

import (
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"net/http"
	"server/protocol/generate/pb"
	"sync/atomic"
)

type proxyWs struct {
	clientConn *websocket.Conn
	cRead      chan []byte
	cWrite     chan []byte
	id         uint32
}

func (r *proxyWs) ReadMsg() {
	Gogo(func() {
		defer func() {
		}()

		for {
			select {
			case data := <-r.cRead:
				if data == nil {
					return
				}
				head := &MessageHead{}
				if err := head.Decode(data[:MsgHeadSize]); err != nil {
					LogError("proxyWs read head err:%v", err)
					return
				}
				// 转发到逻辑服
				rpc := GetProtoService(int32(head.ProtoId))
				if rpc == nil {
					LogError("get proto[%v] service fail", head.ProtoId)
					continue
				}
				rpc.Send(NewBytesMsg(head.ProtoId, head.RoleId, data[MsgHeadSize:]))
			}
		}
	})
}

func (r *proxyWs) WriteMsg() {
	Gogo(func() {
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
					LogError("proxyWs write err:%v", err)
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
		LogError("upgrade err:%v", upgradeErr)
		return
	}

	Gogo(func() {
		// 构造代理对象
		proxy := &proxyWs{
			clientConn: conn,
			cRead:      make(chan []byte, 64),
			cWrite:     make(chan []byte, 64),
			id:         atomic.AddUint32(&proxyId, 1),
		}

		// 首条消息类型指定
		_, firstData, readErr := proxy.clientConn.ReadMessage()
		if readErr != nil {
			LogError("StartProxy read first message err:%v", readErr)
			return
		}
		loginC2S := &pb.LoginC2S{}
		if decodeErr := proto.Unmarshal(firstData[MsgHeadSize:], loginC2S); decodeErr != nil {
			LogError("StartProxy unmarshal first message err:%v", decodeErr)
			return
		}

		// 转发消息到逻辑服
		rpc := GetProtoService(int32(pb.ProtocolId_Login))
		if rpc == nil {
			LogError("get proto service fail")
			return
		}
		rpc.Send(NewProtoMsg(pb.ProtocolId_Login, uint32(loginC2S.RoleId), loginC2S))

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
				LogError("StartProxy read normal message err:%v", err)
				return
			}
			proxy.cRead <- data
		}
	})
}
