package frame

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type tcpMsqQue struct {
	msgQue
	conn       net.Conn
	waitGroup  sync.WaitGroup
	netType    string
	address    string
	connecting int32
}

func (r *tcpMsqQue) Stop() {
	if atomic.CompareAndSwapInt32(&r.stopFlag, 0, 1) {
		r.baseStop()
	}
}

func (r *tcpMsqQue) read() {
	defer func() {
		r.waitGroup.Done()
		if err := recover(); err != nil {
			fmt.Printf("msgQue read panic id:%v err:%v\n", r.uid, err)
		}
		r.Stop()
	}()
	r.waitGroup.Add(1)

	headData := make([]byte, MsgHeadSize)
	var head *MessageHead
	var body []byte

	for !r.IsStop() {
		// 消息头
		if head == nil {
			_, err := io.ReadFull(r.conn, headData)
			if err != nil {
				fmt.Printf("read head err:%v\n", err)
				break
			}
			if head = NewMessageHead(headData); head == nil {
				fmt.Printf("read head head:%v\n", head)
				break
			}
			body = make([]byte, head.Length)
			continue
		}
		// 消息体
		_, err := io.ReadFull(r.conn, body)
		if err != nil {
			fmt.Printf("read head body err:%v\n", err)
			break
		}
		if !r.processMsg(&Message{Head: head, Body: body}) {
			break
		}
		head, body = nil, nil
		r.lastTick = TimeStamp
	}
}

func (r *tcpMsqQue) write() {
	defer func() {
		r.waitGroup.Done()
		if err := recover(); err != nil {
			fmt.Printf("msgQue write panic id:%v err:%v\n", r.uid, err)
		}
		if r.conn != nil {
			_ = r.conn.Close()
		}
		r.Stop()
	}()
	r.waitGroup.Add(1)

	var msg *Message
	var body []byte
	var writePos = 0
	tick := time.NewTimer(time.Second * time.Duration(MsgTimeoutSec))
	for !r.IsStop() {
		if msg == nil {
			select {
			case msg = <-r.writeChannel:
				if msg != nil {
					body = msg.Bytes()
				}
			case <-stopChannel:
				// do nothing
			case <-tick.C:
				if r.isTimeout(tick) {
					r.Stop()
				}
			}
		}

		if msg == nil {
			continue
		}

		// 拆包发送
		if writePos < len(body) {
			n, err := r.conn.Write(body[writePos:])
			if err != nil {
				fmt.Printf("write body err:%v\n", err)
				break
			}
			writePos += n
		}

		if writePos == len(body) {
			writePos = 0
			msg = nil
		}
		r.lastTick = TimeStamp
	}
	tick.Stop()
}

// 主动向对端发起连接
func (r *tcpMsqQue) connect() {
	conn, err := net.DialTimeout(r.netType, r.address, time.Second)
	if err != nil {
		fmt.Printf("tcpMsqQue connect err:%v\n", err)
		atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
		r.Stop()
		return
	}
	r.conn = conn
	atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
	Gogo(func() {
		fmt.Printf("from connect dial tcp[%v] read start\n", r.uid)
		r.read()
		fmt.Printf("from connect dial tcp[%v] read end\n", r.uid)
	})
	Gogo(func() {
		fmt.Printf("from connect dial tcp[%v] write start\n", r.uid)
		r.write()
		fmt.Printf("from connect dial tcp[%v] write end\n", r.uid)
	})
}

func (r *tcpMsqQue) Reconnect(offset int) {
	if r.conn != nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&r.connecting, 0, 1) {
		return
	}
	Gogo(func() {
		r.waitGroup.Wait()
		if offset > 0 {
			time.Sleep(time.Millisecond * time.Duration(offset))
		}
		r.Stop()
		r.connect()
	})
}

// 构造主动连接对象
func newTcpConnect(netType string, address string, handler msgHandler) *tcpMsqQue {
	mq := &tcpMsqQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
			msgHandler:   handler,
		},
		conn:    nil,
		netType: netType,
		address: address,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

// 构造接受连接对象 tcp.Accept
func newTcpAccept(conn net.Conn, handler msgHandler) *tcpMsqQue {
	mq := &tcpMsqQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
			msgHandler:   handler,
		},
		conn: conn,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

func TcpListen(address string, handler msgHandler) error {
	listener, lErr := net.Listen("tcp", address)
	if lErr != nil {
		fmt.Printf("tcp listen err:%v\n", lErr)
		return lErr
	}
	Gogo(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("tcpMsqQue listener accept err:%v\n", err)
				continue
			}
			mq := newTcpAccept(conn, handler)
			Gogo(func() {
				fmt.Printf("from listen accept tcp[%v] read start\n", mq.uid)
				mq.read()
				fmt.Printf("from listen accept tcp[%v] read end\n", mq.uid)
			})
			Gogo(func() {
				fmt.Printf("from listen accept tcp[%v] write start\n", mq.uid)
				mq.write()
				fmt.Printf("from listen accept tcp[%v] write end\n", mq.uid)
			})
		}
	})
	return nil
}
