package frame

import (
	"google.golang.org/protobuf/proto"
	"server/protocol/generate/pb"
	"sync"
	"time"
)

var (
	allLock      = sync.RWMutex{}
	rpcAddr2Mid  = make(map[string]uint32)             // 连接字典
	rpcAddr2Info = make(map[string]*pb.ServerInfo)     // 服务信息
	rpcPid2Addr  = make(map[int32]map[string]struct{}) // 处理路由
)

func GetProtoService(pid int32) IMsgQue {
	allLock.RLock()
	defer allLock.RUnlock()

	set, ok := rpcPid2Addr[pid]
	if !ok {
		return nil
	}
	for addr := range set {
		if mid, ok := rpcAddr2Mid[addr]; ok && MsgQueAvailable(mid) {
			return GetMsgQue(mid)
		}
	}
	return nil
}

func InitRPC() {
	if err := TcpListen(Global.Address, DefaultMsgHandler); err != nil {
		LogError("InitRPC TcpListen Failed Err: %v", err)
		return
	}

	connectServers()
	ticker := time.NewTicker(time.Second * 1)
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
	for _, address := range Global.Servers {
		allLock.RLock()
		mid, ok := rpcAddr2Mid[address]
		allLock.RUnlock()

		if ok && MsgQueAvailable(mid) {
			continue
		}
		LaunchConnect("tcp", address, DefaultMsgHandler, 0)
	}
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
	mq.Send(NewProtoMsg(pb.ProtocolId_ServerHello, 0, serverHello))
}

func HandlerServerHello(mq IMsgQue, body []byte) bool {
	serverHello := &pb.ServerInfo{}
	if err := proto.Unmarshal(body, serverHello); err != nil {
		return false
	}

	allLock.Lock()
	defer allLock.Unlock()

	rpcAddr2Mid[serverHello.Address] = mq.GetUid()
	rpcAddr2Info[serverHello.Address] = serverHello
	for _, pid := range serverHello.MsgHandlers {
		if _, ok := rpcPid2Addr[pid]; !ok {
			rpcPid2Addr[pid] = make(map[string]struct{})
		}
		rpcPid2Addr[pid][serverHello.Address] = struct{}{}
	}
	return true
}
