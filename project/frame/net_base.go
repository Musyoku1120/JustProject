package frame

import (
	"strings"
	"sync/atomic"
	"time"
)

var (
	ConnectTimeoutSec = 300 // 消息超时秒
)

type msgQue struct {
	uid          uint32
	stopFlag     int32 // CAS_Int32
	writeChannel chan *Message
	lastTick     int64
	handler      IMsgHandler
}

func (r *msgQue) Send(msg *Message) (rp bool) {
	if msg == nil || r.stopFlag == 1 {
		return false
	}
	defer func() {
		if err := recover(); err != nil {
			LogPanic()
			rp = false
		}
	}()
	select {
	case r.writeChannel <- msg:
	default:
		LogWarn("msqQueue[%v] channel full", r.uid) // 非阻塞式发送
		r.writeChannel <- msg
	}
	return true
}

func (r *msgQue) isTimeout(tick *time.Timer) bool {
	return false
	//past := int(TimeStamp - r.lastTick)
	//if past < ConnectTimeoutSec {
	//	tick.Reset(time.Second * time.Duration(ConnectTimeoutSec-past))
	//	return false
	//}
	//return true
}

func (r *msgQue) IsStop() bool {
	if r.stopFlag == 0 && ServerEnd() {
		r.Stop()
	}
	return r.stopFlag == 1
}

func (r *msgQue) Stop() {
	if atomic.CompareAndSwapInt32(&r.stopFlag, 0, 1) {
		close(r.writeChannel)
		msgQueLock.Lock()
		delete(msgQueMap, r.uid)
		msgQueLock.Unlock()
	}
}

func (r *msgQue) processMsg(mq IMsgQue, msg *Message) (rp bool) {
	rp = true
	// 同步模式 收到消息即处理
	TryIt(func() {
		//fun := r.handler.GetHandlerFunc(msg.Head.ProtoId)
		//if fun != nil {
		//	rp = fun(msgQue, msg.Body)
		//} else {
		//	rp = false
		//}
		rp = r.handler.OnSolveMsg(mq, msg)
	}, nil)
	return rp
}

func (r *msgQue) GetUid() uint32 {
	return r.uid
}

type IMsgQue interface {
	Send(msg *Message) (rep bool)
	read()
	write()
	GetUid() uint32
}

func MsgQueAvailable(uid uint32) bool {
	msgQueLock.RLock()
	defer msgQueLock.RUnlock()
	_, ok := msgQueMap[uid]
	return ok
}

func GetMsgQue(uid uint32) IMsgQue {
	msgQueLock.RLock()
	defer msgQueLock.RUnlock()
	mq, ok := msgQueMap[uid]
	if ok {
		return mq
	}
	return nil
}

func LaunchConnect(netType string, address string, handler IMsgHandler, delay int32) {
	if strings.Contains(netType, "tcp") {
		mq := newTcpConnect("tcp", address, handler)
		mq.Reconnect(delay) // 立即连接
	}
}
