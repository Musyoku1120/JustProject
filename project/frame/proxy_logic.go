package frame

import (
	"sync"
)

type GameProxy struct {
	roleId  int32
	mq      IMsgQue
	msgLock sync.Mutex
	msgList []*Message
}
