package frame

type HandlerFunc func(body []byte) bool

type MsgHandler struct {
	id2Handler map[int32]HandlerFunc
}

func NewMsgHandler() *MsgHandler {
	return &MsgHandler{
		id2Handler: make(map[int32]HandlerFunc),
	}
}

func (r *MsgHandler) GetHandler(id int32) HandlerFunc {
	fun, ok := r.id2Handler[id]
	if !ok {
		return nil
	}
	return fun
}

func (r *MsgHandler) AddHandler(id int32, fun HandlerFunc) {
	r.id2Handler[id] = fun
}

func (r *MsgHandler) AddHandlers(handlerMap map[int32]HandlerFunc) {
	for id, fun := range handlerMap {
		r.id2Handler[id] = fun
	}
}
