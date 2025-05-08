package frame

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type wsMsgQue struct {
	msgQue
	conn       *websocket.Conn
	waitGroup  sync.WaitGroup
	requestUrl string
	address    string
	connecting int32
}

func (r *wsMsgQue) Stop() {
	if atomic.CompareAndSwapInt32(&r.stopFlag, 0, 1) {
		r.baseStop()
	}
}

func (r *wsMsgQue) read() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("msgQue read panic id:%v err:%v", r.uid, err)
		}
		r.Stop()
	}()

	for !r.IsStop() {
		_, data, err := r.conn.ReadMessage()
		if err != nil {
			fmt.Printf("read message err:%v", err)
			break
		}
		if !r.processMsg(&Message{Body: data}) {
			break
		}
		r.lastTick = TimeStamp
	}
}

func (r *wsMsgQue) write() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("msgQue write panic id:%v err:%v", r.uid, err)
		}
		if r.conn != nil {
			_ = r.conn.Close()
		}
		r.Stop()
	}()

	var msg *Message
	tick := time.NewTimer(time.Second * time.Duration(MsgTimeoutSec))
	for !r.IsStop() {
		if msg == nil {
			select {
			case msg = <-r.writeChannel:
			case <-stopChanForGo:
				// do nothing
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
			fmt.Printf("write message err:%v", err)
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
		fmt.Printf("websocket connect dial err:%v", err)
		atomic.CompareAndSwapInt32(&r.connecting, 0, 1)
		r.Stop()
		return
	}
	r.conn = conn
	atomic.CompareAndSwapInt32(&r.connecting, 0, 1)
	Gogo(func() {
		fmt.Printf("ws[%v] read start", r.uid)
		r.read()
		fmt.Printf("ws[%v] read end", r.uid)
	})
	Gogo(func() {
		fmt.Printf("ws[%v] write start", r.uid)
		r.write()
		fmt.Printf("ws[%v] write end", r.uid)
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
		r.waitGroup.Wait()
		if offset > 0 {
			time.Sleep(time.Millisecond * time.Duration(offset))
		}
		r.Stop()
		r.connect()
	})
}

// 构造主动连接对象
func newWsConnect(address string) *wsMsgQue {
	mq := &wsMsgQue{
		msgQue: msgQue{
			uid:          atomic.AddUint32(&msgQueUId, 1),
			writeChannel: make(chan *Message, 64),
			lastTick:     TimeStamp,
		},
		conn:    nil,
		address: address,
	}
	msgQueLock.Lock()
	msgQueMap[mq.uid] = mq
	msgQueLock.Unlock()
	return mq
}

// 构造接受连接对象 来自http的upgrade
func newWsAccept(conn *websocket.Conn) *wsMsgQue {
	mq := &wsMsgQue{
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

func WsListen(requestUrl string) {
	wsUpgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	http.HandleFunc(requestUrl, func(writer http.ResponseWriter, request *http.Request) {
		conn, err := wsUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			fmt.Printf("upgrade err:%v", err)
			return
		}
		mq := newWsAccept(conn)
		Gogo(func() {
			fmt.Printf("ws[%v] read start", mq.uid)
			mq.read()
			fmt.Printf("ws[%v] read end", mq.uid)
		})
		Gogo(func() {
			fmt.Printf("ws[%v] write start", mq.uid)
			mq.write()
			fmt.Printf("ws[%v] write end", mq.uid)
		})
	})
}
