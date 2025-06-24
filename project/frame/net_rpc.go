package frame

import (
	"google.golang.org/protobuf/proto"
	"server/protocol/generate/pb"
	"sync"
	"time"
)

const rpcTimeoutSecond = 30

var (
	allLock      = sync.Mutex{}
	rpcAddr2Mid  = make(map[string]uint32)            // 连接字典
	rpcPid2Sid   = make(map[int32]map[int32]struct{}) // 处理路由
	rpcAddr2Info = make(map[string]*pb.ServerInfo)    // 服务信息
)

func InitRPC() {
	connectServers()
	ticker := time.NewTicker(time.Second * 3)
	systemGo(func(stopCh chan struct{}) {
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				connectServers()
			}
		}
	})
}

func connectServers() {
	for _, server := range Global.Servers {
		allLock.Lock()
		mid, ok := rpcAddr2Mid[server]
		allLock.Unlock()
		if ok && MsgQueAvailable(mid) {
			continue
		}
		connectServer(server)
	}
}

func connectServer(addr string) {
	mq := newTcpConnect("tcp", addr, DefaultMsgHandler)
	mq.Reconnect(0) // 立即连接
}

func SendServerHello(mq IMsgQue) {
	serverHello := &pb.ServerInfo{
		Id:      Global.UniqueId,
		Name:    Global.ServerName,
		Address: Global.Address,
	}

	for id := range DefaultMsgHandler.id2Handler {
		serverHello.MsgHandlers = append(serverHello.MsgHandlers, id)
	}
	mq.Send(NewPbMsg(pb.ProtocolId_ServerHello, serverHello))
}

func HandlerServerHello(mq IMsgQue, body []byte) bool {
	serverHello := &pb.ServerInfo{}
	if err := proto.Unmarshal(body, serverHello); err != nil {
		return false
	}
	serverHello.LastHeartbeat = TimeStamp

	allLock.Lock()
	rpcAddr2Mid[serverHello.Address] = mq.GetUid()
	rpcAddr2Info[serverHello.Address] = serverHello
	for _, pid := range serverHello.MsgHandlers {
		if _, ok := rpcPid2Sid[pid]; !ok {
			rpcPid2Sid[pid] = make(map[int32]struct{})
		}
		rpcPid2Sid[pid][serverHello.Id] = struct{}{}
	}
	allLock.Unlock()
	return true
}
