package frame

import (
	"os"
	"os/signal"
	"server/protocol/generate/pb"
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
	msgQueUId  uint32                 // 消息队列唯一id
	msgQueMap  = map[uint32]IMsgQue{} // 消息队列字典
	msgQueLock sync.RWMutex           // 消息队列字典锁
)

var Global *ConfigGlobal
var defaultLogger *Log
var DefaultMsgHandler *MsgHandler

var GlobalRedis *RedisManager // 服务发现 消息广播

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

		// 1.关闭网络交互 tcp|ws|udp 2.关闭逻辑业务 goroutine pool
		if !atomic.CompareAndSwapInt32(&globalStop, 0, 1) {
			return
		}
		close(stopChForGo)
		waitAllGo.Wait()

		// 3.关闭日志
		if !atomic.CompareAndSwapInt32(&loggerStop, 0, 1) {
			return
		}
		close(stopChForSys)
		waitAllSys.Wait()

		os.Exit(0)
	}
}

func InitBase() {
	defaultLogger = NewLog(10240, &ConsoleLogger{}, &FileLogger{Path: Global.LogPath})
	defaultLogger.SetLevel(LogLevelError)

	var err error
	GlobalRedis, err = NewRedisManager("127.0.0.1:9999@")
	if err != nil {
		LogPanic("Create RedisManager Failed, Err: %v", err)
		return
	}

	DefaultMsgHandler = NewMsgHandler()
	DefaultMsgHandler.RegisterHandler(int32(pb.ProtocolId_ServerHello), HandlerServerHello)

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
