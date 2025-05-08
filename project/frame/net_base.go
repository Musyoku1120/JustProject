package frame

import "fmt"

type msgQueue struct {
	uid          uint32
	stopFlag     int32 // CAS_Int32
	writeChannel chan *Message
}

func (r *msgQueue) IsStop() bool {
	return r.stopFlag == 1
}

func (r *msgQueue) SendMsg(msg *Message) (rep bool) {
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

func (r *msgQueue) baseStop() {
	if r.writeChannel != nil {
		close(r.writeChannel)
	}
	msgQueLock.Lock()
	delete(msgQueMap, r.uid)
	msgQueLock.Unlock()
}

func (r *msgQueue) processMsg(msg *Message) {
	Gogo(func() {
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
