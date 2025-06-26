package serve

import (
	"server/frame"
	"sync"
)

type GameProxy struct {
	roleId  int32
	mq      frame.IMsgQue
	msgLock sync.Mutex
	msgList []*frame.Message
}
