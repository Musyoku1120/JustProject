package frame

import (
	"sync/atomic"
	"time"
)

type msgQue struct {
	uid          uint32
	stopFlag     int32 // CAS_Int32
	writeChannel chan *Message
	lastTick     int64
	*MsgHandler
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
	past := int(TimeStamp - r.lastTick)
	if past < MsgTimeoutSec {
		tick.Reset(time.Second * time.Duration(MsgTimeoutSec-past))
		return false
	}
	return true
}

func (r *msgQue) IsStop() bool {
	if ServerEnd() {
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
		fun := r.MsgHandler.GetHandler(int32(msg.Head.ProtoId))
		if fun != nil {
			rp = fun(mq, msg.Body)
		} else {
			rp = false
		}
	}, nil)
	return rp
}

type IMsgQue interface {
	Send(msg *Message) (rep bool)
	read()
	write()
}
