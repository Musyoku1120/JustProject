package frame

import (
	"time"
)

type msgQue struct {
	uid          uint32
	stopFlag     int32 // CAS_Int32
	writeChannel chan *Message
	lastTick     int64
	msgHandler
}

func (r *msgQue) IsStop() bool {
	return r.stopFlag == 1
}

func (r *msgQue) SendMsg(msg *Message) (rp bool) {
	if r.IsStop() || msg == nil {
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
		LogInfo("msqQueue[%v] channel full", r.uid)
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

func (r *msgQue) baseStop() {
	if r.writeChannel != nil {
		close(r.writeChannel)
	}
	msgQueLock.Lock()
	delete(msgQueMap, r.uid)
	msgQueLock.Unlock()
}

func (r *msgQue) processMsg(msg *Message) (rp bool) {
	rp = true
	// 同步模式 收到消息即处理
	TryIt(func() {
		fun := r.msgHandler.GetHandler(int32(msg.Head.ProtoId))
		if fun != nil {
			rp = fun(msg.Body)
		} else {
			rp = false
		}
	}, nil)
	return rp
}

type IMsgQueue interface {
	Stop()
	SendMsg(msg *Message) (rep bool)
	read()
	write()
}
