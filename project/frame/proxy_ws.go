package frame

import (
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"net/http"
	"server/protocol/generate/pb"
	"sync"
)

var (
	wsMap  map[int32]*ProxyWs
	wsLock sync.RWMutex

	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

type ProxyWs struct {
	roleId int32

	clientConn *websocket.Conn
	cRead      chan []byte // c2s
	cWrite     chan []byte // s2c
}

func (r *ProxyWs) Solve(msg *Message) {
	r.cWrite <- msg.Bytes()
}

func (r *ProxyWs) ReadMsg() {
	Gogo(func() {
		defer func() {
		}()

		for {
			select {
			case <-stopChForGo:
				return
			case data := <-r.cRead:
				if data == nil {
					return
				}
				if len(data) < MsgHeadSize {
					LogError("ProxyWs read data length err")
					return
				}
				head := &MessageHead{}
				if err := head.Decode(data[:MsgHeadSize]); err != nil {
					LogError("ProxyWs read head err:%v", err)
					return
				}
				if len(data) != int(MsgHeadSize+head.Length) {
					LogError("ProxyWs read data length err")
					return
				}

				// 转发到逻辑服
				rpc := GetProtoService(int32(head.ProtoId))
				if rpc == nil {
					LogError("get proto[%v] service fail", head.ProtoId)
					rpc.Send(NewReplyMsg(r.roleId, &pb.ErrorHint{Hint: "service not found"}))
					continue
				}
				rpc.Send(NewBytesMsg(head.ProtoId, head.RoleId, data[MsgHeadSize:]))
			}
		}
	})
}

func (r *ProxyWs) WriteMsg() {
	Gogo(func() {
		defer func() {
			if r.clientConn != nil {
				_ = r.clientConn.Close()
			}
		}()

		for {
			select {
			case <-stopChForGo:
				return
			case data := <-r.cWrite:
				if data == nil {
					return
				}
				// 回复给玩家客户端
				LogDebug("ProxyWs write data len:%v", len(data))
				err := r.clientConn.WriteMessage(websocket.BinaryMessage, data)
				if err != nil {
					LogError("ProxyWs write err:%v", err)
					return
				}
			}
		}
	})
}

func (r *ProxyWs) Close() {
	_ = r.clientConn.Close()
	r.ReadMsg()
	r.WriteMsg()
	wsLock.Lock()
	delete(wsMap, r.roleId)
	defer wsLock.Unlock()
}

func GetWs(roleId int32) *ProxyWs {
	wsLock.RLock()
	defer wsLock.RUnlock()
	if wsMap == nil {
		wsMap = make(map[int32]*ProxyWs)
	}
	return wsMap[roleId]
}

func GenWs(conn *websocket.Conn, roleId int32) *ProxyWs {
	wsLock.Lock()
	defer wsLock.Unlock()
	if wsMap == nil {
		wsMap = make(map[int32]*ProxyWs)
	}
	wsMap[roleId] = &ProxyWs{
		roleId:     roleId,
		clientConn: conn,
		cRead:      make(chan []byte, 64),
		cWrite:     make(chan []byte, 64),
	}
	return wsMap[roleId]
}

func StartProxy(writer http.ResponseWriter, request *http.Request) {
	conn, upgradeErr := wsUpgrader.Upgrade(writer, request, nil)
	if upgradeErr != nil {
		LogError("upgrade err:%v", upgradeErr)
		return
	}

	Gogo(func() {
		loginC2S := &pb.LoginC2S{}

		firstTry := func() bool {
			// 首条消息类型指定
			_, firstData, readErr := conn.ReadMessage()
			if readErr != nil {
				LogError("StartProxy read first message err:%v", readErr)
				return false
			}
			if len(firstData) < MsgHeadSize {
				LogError("StartProxy first message length err")
				return false
			}
			if decodeErr := proto.Unmarshal(firstData[MsgHeadSize:], loginC2S); decodeErr != nil {
				LogError("StartProxy unmarshal first message err:%v", decodeErr)
				return false
			}
			return true
		}
		if !firstTry() {
			_ = conn.Close()
			return
		}

		proxy := GenWs(conn, loginC2S.RoleId)
		defer proxy.Close()

		// 转发消息到逻辑服
		rpc := GetProtoService(int32(pb.ProtocolId_Login))
		if rpc == nil {
			LogError("get proto service fail")
			return
		}
		rpc.Send(NewProtoMsg(pb.ProtocolId_Login, loginC2S.RoleId, loginC2S))

		// 收发消息
		for {
			select {
			case <-stopChForGo:
				return
			default:
				_, data, err := proxy.clientConn.ReadMessage()
				if err != nil {
					LogError("ws proxy read normal message err:%v", err)
					return
				}
				proxy.cRead <- data
			}
		}
	})
}
