// Code generated by protocol/pb_generate.go on 2025-07-09 10:34:41; DO NOT EDIT.

package generate

import (
	"google.golang.org/protobuf/proto"
	"server/frame"
	"server/protocol/generate/pb"
)

var CSHandlerObj *CSHandler
var CSHandlerMap map[int32]frame.HandlerFunc

func init()  {
	CSHandlerObj = &CSHandler{}
	CSHandlerMap = map[int32]frame.HandlerFunc{
		101 : CSHandlerObj.OnHandlerLogin,
		102 : CSHandlerObj.OnHandlerLogout,
		103 : CSHandlerObj.OnHandlerCommon,
	}
}
type CSHandler struct {
	HandlerLogin func(mq frame.IMsgQue, req *pb.LoginC2S) error
	HandlerLogout func(mq frame.IMsgQue, req *pb.LogoutC2S) error
	HandlerCommon func(mq frame.IMsgQue, req *pb.CommonC2S) error
}

func (h *CSHandler) OnHandlerLogin(mq frame.IMsgQue, body []byte) bool {
	req := &pb.LoginC2S{}
	if err := proto.Unmarshal(body, req); err != nil {
		frame.LogError("Failed to unmarshal protoId: 101, err: %v", err)
		return false
	}
	frame.LogDebug("Received protoId: 101, req: %v", req.String())
	if err := h.HandlerLogin(mq, req); err != nil {
		frame.LogError("Failed to handle protoId: 101, err: %v", err)
		return false
	}
	return true
}

func (h *CSHandler) OnHandlerLogout(mq frame.IMsgQue, body []byte) bool {
	req := &pb.LogoutC2S{}
	if err := proto.Unmarshal(body, req); err != nil {
		frame.LogError("Failed to unmarshal protoId: 102, err: %v", err)
		return false
	}
	frame.LogDebug("Received protoId: 102, req: %v", req.String())
	if err := h.HandlerLogout(mq, req); err != nil {
		frame.LogError("Failed to handle protoId: 102, err: %v", err)
		return false
	}
	return true
}

func (h *CSHandler) OnHandlerCommon(mq frame.IMsgQue, body []byte) bool {
	req := &pb.CommonC2S{}
	if err := proto.Unmarshal(body, req); err != nil {
		frame.LogError("Failed to unmarshal protoId: 103, err: %v", err)
		return false
	}
	frame.LogDebug("Received protoId: 103, req: %v", req.String())
	if err := h.HandlerCommon(mq, req); err != nil {
		frame.LogError("Failed to handle protoId: 103, err: %v", err)
		return false
	}
	return true
}
