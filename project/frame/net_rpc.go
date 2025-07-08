package frame

import (
	"google.golang.org/protobuf/proto"
	"server/protocol/generate/pb"
	"sync"
	"time"
)

type rpcServer struct {
	msqQue  IMsgQue
	payload int32
	*pb.ServerInfo
}

var (
	allLock   = sync.RWMutex{}
	linkCheck = make(map[string]uint32)
	pidServes = make(map[int32]map[string]*rpcServer)
)

func GetProtoService(pid int32) IMsgQue {
	allLock.RLock()
	defer allLock.RUnlock()

	set, ok := pidServes[pid]
	if !ok {
		return nil
	}
	for _, service := range set {
		if MsgQueAvailable(service.msqQue.GetUid()) {
			return service.msqQue
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
	Gogo(func() {
		for {
			select {
			case <-stopChForGo:
				return
			case <-ticker.C:
				connectServers()
			}
		}
	})
}

func connectServers() {
	for _, address := range Global.ServerAddr {
		allLock.RLock()
		mid, ok := linkCheck[address]
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
		Type:    Global.ServerType,
		Address: Global.Address,
	}

	for id := range DefaultMsgHandler.id2Handler {
		serverHello.Services = append(serverHello.Services, id)
	}
	mq.Send(NewProtoMsg(pb.ProtocolId_ServerHello, 0, serverHello))
}

func HandlerServerHello(mq IMsgQue, body []byte) bool {
	serverInfo := &pb.ServerInfo{}
	if err := proto.Unmarshal(body, serverInfo); err != nil {
		return false
	}

	allLock.Lock()
	defer allLock.Unlock()

	linkCheck[serverInfo.Address] = mq.GetUid()
	service := &rpcServer{msqQue: mq, ServerInfo: serverInfo}
	for _, pid := range service.Services {
		if _, ok := pidServes[pid]; !ok {
			pidServes[pid] = make(map[string]*rpcServer)
		}
		pidServes[pid][serverInfo.Address] = service
	}
	return true
}
