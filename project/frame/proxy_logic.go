package frame

import (
	"sync"
)

type proxyLogic struct {
	roleId  int32
	mq      IMsgQue
	msgLock sync.Mutex
	msgList []*Message
}
