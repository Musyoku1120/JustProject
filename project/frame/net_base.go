package frame

import "fmt"

type msgQueue struct {
	uid          uint32
	stopFlag     int32 // CAS_Int32
	writeChannel chan *Message
}

func (r *msgQueue) SendMsg(msg *Message) (rep bool) {
	if r.stopFlag == 1 || msg == nil {
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

func (r *msgQueue) processMsg(msg *Message) {
	AsyncDo(func() {
		r.processMsgReal(msg)
	})
}

func (r *msgQueue) processMsgReal(msg *Message) {

}

type IMsgQueue interface {
	Stop()
	SendMsg(msg *Message) (rep bool)
	read()
	write()
}
