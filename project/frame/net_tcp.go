package frame

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type tcpMsqQue struct {
	msgQue
	conn       *net.TCPConn
	waitGroup  sync.WaitGroup
	netType    string
	address    string
	connecting int32
}

func (r *tcpMsqQue) read() {
	defer func() {
		r.waitGroup.Done()
		if err := recover(); err != nil {
			LogPanic()
		}
		r.Stop()
	}()
	r.waitGroup.Add(1)

	headData := make([]byte, MsgHeadSize)
	var head *MessageHead
	var body []byte

	for !r.IsStop() {
		// 消息头
		_, err := io.ReadFull(r.conn, headData)
		if err != nil {
			if err != io.EOF {
				LogError("receive data err:%v", err)
			}
			LogError("read head err:%v", err)
			break
		}
		if head = NewMessageHead(headData); head == nil {
			LogError("read head nil:%v", head)
			break
		}
		LogDebug("receive head:%v", head)

		// 消息体
		body = make([]byte, head.Length)
		_, err = io.ReadFull(r.conn, body)
		if err != nil {
			LogError("read head body err:%v", err)
			break
		}
		LogDebug("receive body:%v", body)
		if !r.processMsg(r, &Message{Head: head, Body: body}) {
			LogWarn("server[%v] process[%v] failed", Global.UniqueId, head.ProtoId)
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
			LogError("msgQue write panic id:%v err:%v", r.uid, err)
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
	tick := time.NewTimer(time.Second * time.Duration(ConnectTimeoutSec))
	for !r.IsStop() || msg != nil {
		if msg == nil {
			select {
			case <-stopChForGo:
			case msg = <-r.writeChannel:
				if msg != nil {
					body = msg.Bytes()
				}
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
				LogError("write body err:%v", err)
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
		LogError("tcpMsqQue connect err:%v", err)
		atomic.CompareAndSwapInt32(&r.connecting, 1, 0)
		r.Stop()
		return
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		r.conn = tcpConn
	}
	atomic.CompareAndSwapInt32(&r.connecting, 1, 0)

	_ = r.conn.SetKeepAlive(true)
	_ = r.conn.SetKeepAlivePeriod(30 * time.Second)

	Gogo(func() {
		LogInfo("from connect dial tcp[%v] read start", r.uid)
		r.read()
		LogInfo("from connect dial tcp[%v] read end", r.uid)
	})
	Gogo(func() {
		LogInfo("from connect dial tcp[%v] write start", r.uid)
		r.write()
		LogInfo("from connect dial tcp[%v] write end", r.uid)
	})
}

func (r *tcpMsqQue) Reconnect(offset int32) {
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
		r.stopFlag = 0 // 关闭 停止标识
		r.connect()
	})
}

// 构造主动连接对象
func newTcpConnect(netType string, address string, handler IMsgHandler) *tcpMsqQue {
	mq := &tcpMsqQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
			handler:      handler,
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
func newTcpAccept(conn *net.TCPConn, handler IMsgHandler) *tcpMsqQue {
	mq := &tcpMsqQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
			handler:      handler,
		},
		conn: conn,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

func TcpListen(address string, handler IMsgHandler) error {
	tcpAddr, rErr := net.ResolveTCPAddr("tcp", address)
	if rErr != nil {
		LogError("tcp addr resolve err:%v", rErr)
		return rErr
	}
	listener, lErr := net.ListenTCP("tcp", tcpAddr)
	if lErr != nil {
		LogError("tcp listen err:%v", lErr)
		return lErr
	}

	go func() {
		defer func() {
			_ = listener.Close()
		}()
		for {
			conn, err := listener.AcceptTCP() // 阻塞式 单独启动一个goroutine
			if err != nil {
				LogError("tcpMsqQue listener accept err:%v", err)
				continue
			}
			mq := newTcpAccept(conn, handler)

			mq.handler.OnConnComplete(mq)

			Gogo(func() {
				LogInfo("from listen accept tcp[%v] read start", mq.uid)
				mq.read()
				LogInfo("from listen accept tcp[%v] read end", mq.uid)
			})
			Gogo(func() {
				LogInfo("from listen accept tcp[%v] write start", mq.uid)
				mq.write()
				LogInfo("from listen accept tcp[%v] write end", mq.uid)
			})
		}
	}()
	return nil
}
