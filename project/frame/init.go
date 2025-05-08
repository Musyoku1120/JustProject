package frame

import (
	"sync"
)

var waitAll = &sync.WaitGroup{} // 等待所有goroutine
var stopChanForGo = make(chan struct{})

var (
	goCount int32 // goroutine数量
	goUid   uint32
)

var (
	poolChan    = make(chan func())
	poolGoCount int32
	poolSize    int32 = 100
)

var (
	msgQueUId  uint32 // 消息队列唯一id
	msgQueMap  = map[uint32]IMsgQueue{}
	msgQueLock sync.Mutex
)
