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
	listener   net.Listener
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
			fmt.Printf("msgQue read panic id:%v err:%v", r.uid, err)
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
				fmt.Printf("read head err:%v", err)
				break
			}
			if head = NewMessageHead(headData); head == nil {
				fmt.Printf("read head head:%v", head)
				break
			}
			body = make([]byte, head.Length)
			continue
		}
		// 消息体
		_, err := io.ReadFull(r.conn, body)
		if err != nil {
			fmt.Printf("read head body err:%v", err)
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
	var writePos = 0
	tick := time.NewTimer(time.Second * time.Duration(MsgTimeoutSec))
	for !r.IsStop() {
		if msg == nil {
			select {
			case msg = <-r.writeChannel:
				if msg != nil {
					body = msg.Bytes()
				}
			case <-stopChanForGo:
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
				fmt.Printf("write body err:%v", err)
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
		fmt.Printf("tcpMsqQue connect err:%v", err)
		atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
		r.Stop()
		return
	}
	r.conn = conn
	atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
	Gogo(func() {
		fmt.Printf("tcp[%v] read start", r.uid)
		r.read()
		fmt.Printf("tcp[%v] read end", r.uid)
	})
	Gogo(func() {
		fmt.Printf("tcp[%v] write start", r.uid)
		r.write()
		fmt.Printf("tcp[%v] write end", r.uid)
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
func newTcpConnect(conn net.Conn, netType string, address string) *tcpMsqQue {
	mq := &tcpMsqQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
		},
		conn:    conn,
		netType: netType,
		address: address,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

// 构造接受连接对象 tcp.Accept
func newTcpAccept(conn net.Conn) *tcpMsqQue {
	mq := &tcpMsqQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
		},
		conn: conn,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

func TcpListen(address string) error {
	listener, lErr := net.Listen("tcp", address)
	if lErr != nil {
		fmt.Printf("tcp listen err:%v", lErr)
		return lErr
	}
	Gogo(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("tcpMsqQue listener accept err:%v", err)
				continue
			}
			mq := newTcpAccept(conn)
			Gogo(func() {
				fmt.Printf("tcp[%v] read start", mq.uid)
				mq.read()
				fmt.Printf("tcp[%v] read end", mq.uid)
			})
			Gogo(func() {
				fmt.Printf("tcp[%v] write start", mq.uid)
				mq.write()
				fmt.Printf("tcp[%v] write end", mq.uid)
			})
		}
	})
	return nil
}
