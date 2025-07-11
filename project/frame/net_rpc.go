package frame

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"server/protocol/generate/pb"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rpcServer struct {
	msqQue  IMsgQue
	payload int32
	*pb.ServerInfo
}

var (
	allLock    = sync.RWMutex{}
	linkCheck  = make(map[string]uint32)
	pidServe   = make(map[int32]string)
	typeServes = make(map[string]map[int32]*rpcServer)
)

func GetProtoService(pid int32) IMsgQue {
	allLock.RLock()
	defer allLock.RUnlock()

	serveType, ok := pidServe[pid]
	if !ok {
		return nil
	}
	for _, service := range typeServes[serveType] {
		if MsgQueAvailable(service.msqQue.GetUid()) {
			return service.msqQue
		}
	}
	return nil
}

func InitRPC() {
	listenStart := false
	addr := strings.Split(Global.Address, ":")
	for port, _ := strconv.Atoi(addr[1]); port < 65535; port++ {
		err := TcpListen(fmt.Sprintf("%v:%v", addr[0], port), DefaultMsgHandler)
		if err == nil {
			listenStart = true
			Global.Address = fmt.Sprintf("%v:%v", addr[0], port)
			break
		}
	}
	if !listenStart {
		LogError("InitRPC TcpListen By Port Traverse Failed")
		return
	}

	tryConnectServers()
	ticker := time.NewTicker(time.Second * 1)
	Gogo(func() {
		for {
			select {
			case <-stopChForGo:
				deleteServerInfo()
				return
			case <-ticker.C:
				tryConnectServers()
			}
		}
	})
}

func deleteServerInfo() {
	GlobalRedis.HDel(GlobalRedis.ctx, "server.info", fmt.Sprintf("%v", Global.UniqueId))
}

func tryConnectServers() {
	uploadServerInfo()
	servers := lookupServers()
	for _, server := range servers {
		needLink := false
		for _, link := range Global.ServerLinks {
			if link == server.Type {
				needLink = true
				break
			}
		}
		if !needLink {
			continue
		}

		allLock.RLock()
		mid, ok := linkCheck[server.Address]
		allLock.RUnlock()

		if ok && MsgQueAvailable(mid) {
			continue
		}
		LaunchConnect("tcp", server.Address, DefaultMsgHandler, 0)
	}
}

func lookupServers() []*pb.ServerInfo {
	serverStr, err := GlobalRedis.HVals(GlobalRedis.ctx, "server.info").Result()
	if err != nil {
		LogError("lookupServers HVals Failed Err: %v", err)
		return nil
	}
	var servers []*pb.ServerInfo
	for _, str := range serverStr {
		tmp := &pb.ServerInfo{}
		if err := proto.Unmarshal([]byte(str), tmp); err != nil {
			LogError("lookupServers Unmarshal Failed Err: %v", err)
			continue
		}
		servers = append(servers, tmp)
	}
	return servers
}

func uploadServerInfo() {
	serverInfo := &pb.ServerInfo{
		Id:      Global.UniqueId,
		Type:    Global.ServerType,
		Address: Global.Address,
	}
	data, err := proto.Marshal(serverInfo)
	if err != nil {
		LogError("uploadServerInfo Marshal Failed Err: %v", err)
		return
	}
	_, err = GlobalRedis.HSet(GlobalRedis.ctx, "server.info", fmt.Sprintf("%v", serverInfo.Id), data).Result()
	if err != nil {
		LogError("uploadServerInfo HSet Failed Err: %v", err)
		return
	}
}

func SendServerHello(mq IMsgQue) {
	serverInfo := &pb.ServerInfo{
		Id:      Global.UniqueId,
		Type:    Global.ServerType,
		Address: Global.Address,
	}

	for id := range DefaultMsgHandler.id2Handler {
		serverInfo.Services = append(serverInfo.Services, id)
	}
	mq.Send(NewProtoMsg(pb.ProtocolId_ServerHello, 0, serverInfo))
}

func HandlerServerHello(mq IMsgQue, body []byte) bool {
	serverInfo := &pb.ServerInfo{}
	if err := proto.Unmarshal(body, serverInfo); err != nil {
		return false
	}

	allLock.Lock()
	defer allLock.Unlock()

	linkCheck[serverInfo.Address] = mq.GetUid()
	if _, ok := typeServes[serverInfo.Type]; !ok {
		typeServes[serverInfo.Type] = make(map[int32]*rpcServer)
	}
	typeServes[serverInfo.Type][serverInfo.Id] = &rpcServer{
		msqQue:     mq,
		ServerInfo: serverInfo,
	}
	for _, pid := range serverInfo.Services {
		pidServe[pid] = serverInfo.Type
	}
	return true
}
