package frame

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var TimeStamp int64 // 时间戳

var (
	goCount     int32                     // 协程数量
	goUid       uint32                    // 递增唯一id
	poolChan    = make(chan func())       // 任务池 无缓冲区
	poolGoCount int32                     // 协程池协程数
	poolSize    int32               = 100 // 协程池大小

	globalStop      int32
	loggerStop      int32
	waitAllGo       = &sync.WaitGroup{}       // 等待 消费者
	stopChForGo     = make(chan struct{})     // 暂停 消费者
	waitAllSys      = &sync.WaitGroup{}       // 等待 系统
	stopChForSys    = make(chan struct{})     // 暂停 系统
	stopChForSignal = make(chan os.Signal, 1) // 操作系统退出信号
)

var (
	msgQueUId  uint32                   // 消息队列唯一id
	msgQueMap  = map[uint32]IMsgQueue{} // 消息队列字典
	msgQueLock sync.Mutex               // 消息队列字典锁
)

var defaultLogger *Log

func ServerEnd() bool {
	return globalStop == 1
}

func LogEnd() bool {
	return loggerStop == 1
}

func WaitForExit() {
	signal.Notify(stopChForSignal, os.Interrupt, os.Kill, syscall.SIGTERM)
	select {
	case <-stopChForSignal:
		close(stopChForSignal)

		// 关闭网络交互
		if !atomic.CompareAndSwapInt32(&globalStop, 0, 1) {
			return
		}

		// 关闭逻辑业务
		close(stopChForGo)
		waitAllGo.Wait()

		// 关闭系统服务
		if !atomic.CompareAndSwapInt32(&loggerStop, 0, 1) {
			return
		}
		close(stopChForSys)
		waitAllSys.Wait()
		os.Exit(0)
	}
}

func init() {
	defaultLogger = NewLog(10240, &ConsoleLogger{}, &FileLogger{Path: "D:/ServerLog/server.log"})
	defaultLogger.SetLevel(LogLevelError)

	Gogo(func() {
		TimeStamp = time.Now().UnixNano() / 1e9
		var ticker = time.NewTicker(333 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopChForGo:
				return
			case <-ticker.C:
				TimeStamp = time.Now().UnixNano() / 1e9
			}
		}
	})
}
