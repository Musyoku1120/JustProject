package frame

import (
	"google.golang.org/protobuf/proto"
	"server/protocol/generate/pb"
	"time"
)

var (
	rpcId2MQ = make(map[int32]IMsgQue)
	//rpcId2Info = make(map[int32]*pb.ServerInfo)
	rpcPid2Ids = make(map[int32][]int32)
)

func InitRPC() {
	ticker := time.NewTicker(time.Second * 5)
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
	connectServers()
}

func connectServers() {
	servers := []string{
		Global.Servers.GateAddress,
		Global.Servers.GameAddress,
	}

	for _, server := range servers {
		if server == "" {
			continue
		}
		connectServer(server)
	}
}

func connectServer(addr string) {
	mq := newTcpConnect("tcp", addr, defaultMsgHandler)
	mq.Reconnect(0) // 立即连接
	onRpcConnect(mq)
}

func onRpcConnect(mq *tcpMsqQue) {
	serverHello := &pb.ServerInfo{
		Id:   Global.UniqueId,
		Name: Global.ServerName,
	}
	switch Global.ServerName {
	case "gate":
		serverHello.Address = Global.Servers.GateAddress
	case "game":
		serverHello.Address = Global.Servers.GameAddress
	}

	for id := range defaultMsgHandler.id2Handler {
		serverHello.MsgHandlers = append(serverHello.MsgHandlers, id)
	}
	mq.Send(NewRpcMsg(serverHello))
}

func HandlerServerHello(mq IMsgQue, body []byte) bool {
	serverHello := &pb.ServerInfo{}
	if err := proto.Unmarshal(body, serverHello); err != nil {
		return false
	}

	rpcId2MQ[serverHello.Id] = mq
	//rpcId2Info[serverHello.Id] = serverHello
	for _, pid := range serverHello.MsgHandlers {
		rpcPid2Ids[pid] = append(rpcPid2Ids[pid], serverHello.Id)
	}
	return true
}
