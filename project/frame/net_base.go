package frame

import (
	"fmt"
	"time"
)

type msgQue struct {
	uid          uint32
	stopFlag     int32 // CAS_Int32
	writeChannel chan *Message
	lastTick     int64
}

func (r *msgQue) IsStop() bool {
	return r.stopFlag == 1
}

func (r *msgQue) SendMsg(msg *Message) (rep bool) {
	if r.IsStop() || msg == nil {
		return false
	}
	defer func() {
		if err := recover(); err != nil {
			rep = false
		}
	}()
	select {
	case r.writeChannel <- msg:
	default:
		fmt.Printf("msqQueue[%v] channel full\n", r.uid)
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

func (r *msgQue) processMsg(msg *Message) bool {
	Gogo(func() {
		//r.processMsgReal(msg)
	})
	return true
}

type IMsgQueue interface {
	Stop()
	SendMsg(msg *Message) (rep bool)
	read()
	write()
}
