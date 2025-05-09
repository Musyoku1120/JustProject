package frame

type HandlerFunc func(body []byte) bool

type msgHandler struct {
	id2Handler map[int32]HandlerFunc
}

func (r *msgHandler) GetHandler(id int32) HandlerFunc {
	fun, ok := r.id2Handler[id]
	if !ok {
		return nil
	}
	return fun
}
