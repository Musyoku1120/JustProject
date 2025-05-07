package frame

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type tcpMsqQueue struct {
	msgQueue
	conn       net.Conn
	listener   net.Listener
	waitGroup  sync.WaitGroup
	netType    string
	address    string
	connecting int32
}

func (r *tcpMsqQueue) Stop() {
	if atomic.CompareAndSwapInt32(&r.stopFlag, 0, 1) {

	}
}

func (r *tcpMsqQueue) read() {
	defer func() {
		r.waitGroup.Done()
		if err := recover(); err != nil {
			fmt.Printf("msgQue read panic id:%v err:%v", r.uid, err)
		}
		r.Stop()
	}()
	r.waitGroup.Add(1)

	headData := make([]byte, MsgHeadSize)
	var head *MessageHead
	var body []byte

	for r.stopFlag == 0 {
		if head == nil {
			_, err := io.ReadFull(r.conn, headData)
			if err != nil {
				fmt.Printf("read head err:%v", err)
				break
			}
			if head = NewMessageHead(headData); head == nil {
				fmt.Printf("read head head:%v", head)
				break
			}
			body = make([]byte, head.Length)

		} else {
			_, err := io.ReadFull(r.conn, body)
			if err != nil {
				fmt.Printf("read head body err:%v", err)
				break
			}
			r.processMsg(&Message{Head: head, Body: body})
			head, body = nil, nil
		}
	}
}

func (r *tcpMsqQueue) write() {
	defer func() {
		r.waitGroup.Done()
		if err := recover(); err != nil {
			fmt.Printf("msgQue write panic id:%v err:%v", r.uid, err)
		}
		if r.conn != nil {
			_ = r.conn.Close()
		}
		r.Stop()
	}()
	r.waitGroup.Add(1)

	var msg *Message
	var body []byte
	var writePos int = 0
	tick := time.NewTimer(time.Second * time.Duration(MsgTimeoutSec))
	for r.stopFlag == 0 {
		if msg == nil {
			select {
			case <-stopChanForGo:
				// do nothing
			case msg = <-r.writeChannel:
				if msg != nil {
					body = msg.Bytes()
				}
			case <-tick.C:
				r.Stop()
			}
		}

		if msg == nil {
			continue
		}

		if writePos < len(body) {
			n, err := r.conn.Write(body[writePos:]) // 分包发送
			if err != nil {
				fmt.Printf("write body err:%v", err)
				break
			}
			writePos += n
		}

		if writePos == len(body) {
			writePos = 0
			msg = nil
		}
	}
	tick.Stop()
}

// 监听来自远端的连接
func (r *tcpMsqQueue) listen() {
	for r.stopFlag == 0 {
		conn, err := r.listener.Accept()
		if err != nil {
			fmt.Printf("tcpMsqQueue listener accept err:%v", err)
			break
		} else {
			AsyncDo(func() {
				mq := newTcpAccept(conn)
				AsyncDo(func() {
					fmt.Printf("tcp[%v] read start", mq.uid)
					mq.read()
					fmt.Printf("tcp[%v] read end", mq.uid)
				})
				AsyncDo(func() {
					fmt.Printf("tcp[%v] write start", mq.uid)
					mq.write()
					fmt.Printf("tcp[%v] write end", mq.uid)
				})
			})
		}
	}
	r.Stop()
}

// 主动向对端发起连接
func (r *tcpMsqQueue) connect() {
	conn, err := net.DialTimeout(r.netType, r.address, time.Second)
	if err != nil {
		fmt.Printf("tcpMsqQueue connect err:%v", err)
		atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
		r.Stop()
		return
	}
	r.conn = conn
	atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
	AsyncDo(func() {
		fmt.Printf("tcp[%v] read start", r.uid)
		r.read()
		fmt.Printf("tcp[%v] read end", r.uid)
	})
	AsyncDo(func() {
		fmt.Printf("tcp[%v] write start", r.uid)
		r.write()
		fmt.Printf("tcp[%v] write end", r.uid)
	})
}

func (r *tcpMsqQueue) Reconnect(offset int) {
	if r.conn != nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&r.connecting, 0, 1) {
		return
	}
	AsyncDo(func() {
		r.waitGroup.Wait()
		if offset > 0 {
			time.Sleep(time.Millisecond * time.Duration(offset))
		}
		r.stopFlag = 0
		r.connect()
	})
}

func newTcpConnect(conn net.Conn, netType string, address string) *tcpMsqQueue {
	mq := &tcpMsqQueue{
		msgQueue: msgQueue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			stopFlag:     0,
			writeChannel: make(chan *Message, 64),
		},
		conn:      conn,
		waitGroup: sync.WaitGroup{},
		netType:   netType,
		address:   address,
	}
	return mq
}

func newTcpAccept(conn net.Conn) *tcpMsqQueue {
	mq := &tcpMsqQueue{
		msgQueue: msgQueue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			stopFlag:     0,
			writeChannel: make(chan *Message, 64),
		},
		conn: conn,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}
