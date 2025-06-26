package frame

import (
	"github.com/gorilla/websocket"
	"sync/atomic"
	"time"
)

type wsMsgQue struct {
	msgQue
	conn       *websocket.Conn
	requestUrl string
	address    string
	connecting int32
}

func (r *wsMsgQue) read() {
	defer func() {
		if err := recover(); err != nil {
			LogError("msgQue read panic id:%v err:%v", r.uid, err)
		}
		r.Stop()
	}()

	for !r.IsStop() {
		_, data, err := r.conn.ReadMessage()
		if err != nil {
			LogError("read message err:%v", err)
			break
		}
		if !r.processMsg(r, &Message{Body: data}) {
			break
		}
		r.lastTick = TimeStamp
	}
}

func (r *wsMsgQue) write() {
	defer func() {
		if err := recover(); err != nil {
			LogError("msgQue write panic id:%v err:%v", r.uid, err)
		}
		if r.conn != nil {
			_ = r.conn.Close()
		}
		r.Stop()
	}()

	var msg *Message
	tick := time.NewTimer(time.Second * time.Duration(MsgTimeoutSec))
	for !r.IsStop() || msg != nil {
		if msg == nil {
			select {
			case <-stopChForGo:
			case msg = <-r.writeChannel:
			case <-tick.C:
				if r.isTimeout(tick) {
					r.Stop()
				}
			}
		}

		if msg == nil || msg.Body == nil {
			msg = nil
			continue
		}

		err := r.conn.WriteMessage(websocket.BinaryMessage, msg.Body)
		if err != nil {
			LogError("write message err:%v", err)
			break
		}
		msg = nil
		r.lastTick = TimeStamp
	}
	tick.Stop()
}

func (r *wsMsgQue) connect() {
	conn, _, err := websocket.DefaultDialer.Dial(r.address, nil)
	if err != nil {
		LogError("websocket connect dial err:%v", err)
		atomic.CompareAndSwapInt32(&r.connecting, 0, 1)
		r.Stop()
		return
	}
	r.conn = conn
	atomic.CompareAndSwapInt32(&r.connecting, 0, 1)
	Gogo(func() {
		LogInfo("from connect dial ws[%v] read start", r.uid)
		r.read()
		LogInfo("from connect dial ws[%v] read end", r.uid)
	})
	Gogo(func() {
		LogInfo("from connect dial ws[%v] write start", r.uid)
		r.write()
		LogInfo("from connect dial ws[%v] write end", r.uid)
	})
}

func (r *wsMsgQue) Reconnect(offset int) {
	if r.conn != nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&r.connecting, 0, 1) {
		return
	}
	Gogo(func() {
		if offset > 0 {
			time.Sleep(time.Millisecond * time.Duration(offset))
		}
		r.Stop()
		r.connect()
	})
}

// 构造主动连接对象
func newWsConnect(address string, handler IMsgHandler) *wsMsgQue {
	mq := &wsMsgQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
			handler:      handler,
		},
		conn:    nil,
		address: address,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

// 构造接受连接对象
func newWsAccept(conn *websocket.Conn, handler IMsgHandler) *wsMsgQue {
	mq := &wsMsgQue{
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
