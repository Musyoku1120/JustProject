package frame

import "server/protocol/generate/pb"

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

	mq.Send(nil)
}
