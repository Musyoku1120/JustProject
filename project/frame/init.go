package frame

import (
	"sync"
)

var TimeStamp int64                   // 时间戳
var waitAll = &sync.WaitGroup{}       // 等待所有goroutine
var stopChannel = make(chan struct{}) // 暂停所有goroutine

var (
	goCount     int32                     // 协程数量
	goUid       uint32                    // 递增唯一id
	poolChan    = make(chan func())       // 任务池
	poolGoCount int32                     // 协程池协程数
	poolSize    int32               = 100 // 协程池大小
)

var (
	msgQueUId  uint32                   // 消息队列唯一id
	msgQueMap  = map[uint32]IMsgQueue{} // 消息队列字典
	msgQueLock sync.Mutex               // 消息队列字典锁
)

var defaultLogger *Log

func init() {
	defaultLogger = NewLog(10000, &ConsoleLogger{})
}
