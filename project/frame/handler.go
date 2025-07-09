package frame

import "server/protocol/generate/pb"

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

func (r *MsgHandler) OnSolveMsg(mq IMsgQue, msg *Message) bool {
	if msg.Head.ProtoId == pb.ProtocolId_ServerHello {
		return HandlerServerHello(mq, msg.Body)
	}

	switch Global.ServerType {
	case ServerTypeGate:
		gamer := GetWs(msg.Head.RoleId)
		if gamer == nil {
			LogError("gate websocket not found, roleId=%d", msg.Head.RoleId)
			return false
		}
		gamer.Solve(msg)
		return true

	case ServerTypeGame:
		fun := r.GetHandlerFunc(msg.Head.ProtoId)
		if fun == nil {
			// TODO RPC
			LogError("handler not found, protoId=%d", msg.Head.ProtoId)
			mq.Send(NewReplyMsg(msg.Head.RoleId, &pb.ErrorHint{Hint: "handler not found"}))
			return false
		}
		if msg.Head.ProtoId == pb.ProtocolId_Login {
			if old := GetLogic(msg.Head.RoleId); old != nil {
				old.msgQue.Send(NewReplyMsg(old.roleId, &pb.ErrorHint{Hint: "server kick out"}))
				DelLogic(msg.Head.RoleId)
			}
		}
		gamer := GenLogic(msg.Head.RoleId, mq)
		gamer.Solve(msg, fun)
		return true

	default:
		LogPanic("server type not configured")
		return false
	}
}

func (r *MsgHandler) OnConnComplete(mq IMsgQue) bool {
	SendServerHello(mq)
	return true
}

func (r *MsgHandler) GetHandlerFunc(pid pb.ProtocolId) HandlerFunc {
	fun, ok := r.id2Handler[int32(pid)]
	if !ok {
		return nil
	}
	return fun
}

type IMsgHandler interface {
	OnNewMsgQue(mq IMsgQue) bool
	OnDelMsgQue(mq IMsgQue)
	OnSolveMsg(mq IMsgQue, msg *Message) bool
	OnConnComplete(mq IMsgQue) bool
	GetHandlerFunc(pid pb.ProtocolId) HandlerFunc
}
