package frame

import (
	"sync"
)

var (
	proxyMap  map[int32]*ProxyLogic
	proxyLock sync.RWMutex
)

type ProxyLogic struct {
	roleId int32

	serveLock sync.Mutex
	msgLock   sync.Mutex
	msgList   []*Message
}

func (r *ProxyLogic) Solve(msg *Message, msgQue IMsgQue, handler HandlerFunc) {
	if !MsgQueAvailable(msgQue.GetUid()) {
		return
	}

	r.msgLock.Lock()
	r.msgList = append(r.msgList, msg)
	r.msgLock.Unlock()

	Gogo(func() {
		r.serveLock.Lock()
		defer r.serveLock.Unlock()

		r.msgLock.Lock()
		thisMsg := r.msgList[0]
		r.msgList = r.msgList[1:]
		r.msgLock.Unlock()

		TryIt(func() {
			handler(msgQue, thisMsg.Body)
		}, func(err interface{}) {
			msgQue.Send(nil) // fail msg
		})
	})
}

func DelProxy(roleId int32) {
	proxyLock.Lock()
	defer proxyLock.Unlock()
	if proxyMap == nil {
		return
	}
	delete(proxyMap, roleId)
}

func GetProxy(roleId int32) *ProxyLogic {
	proxyLock.RLock()
	defer proxyLock.RUnlock()
	if proxyMap == nil {
		proxyMap = make(map[int32]*ProxyLogic)
	}
	return proxyMap[roleId]
}

func GenProxy(roleId int32) *ProxyLogic {
	proxyLock.Lock()
	defer proxyLock.Unlock()
	if proxyMap == nil {
		proxyMap = make(map[int32]*ProxyLogic)
	}
	if _, ok := proxyMap[roleId]; !ok {
		proxyMap[roleId] = &ProxyLogic{
			roleId: roleId,
		}
	}
	return proxyMap[roleId]
}
