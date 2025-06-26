package frame

type HandlerFunc func(mq IMsgQue, body []byte) bool

type MsgHandler struct {
	id2Handler map[int32]HandlerFunc
}

func NewMsgHandler() *MsgHandler {
	return &MsgHandler{
		id2Handler: make(map[int32]HandlerFunc),
	}
}

func (r *MsgHandler) RegisterHandler(id int32, fun HandlerFunc) {
	r.id2Handler[id] = fun
}

func (r *MsgHandler) RegisterHandlers(handlerMap map[int32]HandlerFunc) {
	for id, fun := range handlerMap {
		r.id2Handler[id] = fun
	}
}

func (r *MsgHandler) OnNewMsgQue(mq IMsgQue) bool {
	return true
}

func (r *MsgHandler) OnDelMsgQue(mq IMsgQue) {
}

func (r *MsgHandler) OnSolveMsg(mq IMsgQue, body []byte) bool {
	return true
}

func (r *MsgHandler) OnConnComplete(mq IMsgQue) bool {
	SendServerHello(mq)
	return true
}

func (r *MsgHandler) GetHandlerFunc(pid int32) HandlerFunc {
	fun, ok := r.id2Handler[pid]
	if !ok {
		return nil
	}
	return fun
}

type IMsgHandler interface {
	OnNewMsgQue(mq IMsgQue) bool
	OnDelMsgQue(mq IMsgQue)
	OnSolveMsg(mq IMsgQue, body []byte) bool
	OnConnComplete(mq IMsgQue) bool
	GetHandlerFunc(pid int32) HandlerFunc
}
