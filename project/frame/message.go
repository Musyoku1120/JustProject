package frame

import (
	"bytes"
	"encoding/binary"
	"errors"
	"google.golang.org/protobuf/proto"
	"server/protocol/generate/pb"
)

const (
	MsgTimeoutSec  = 200         // 消息超时秒
	MsgHeadSize    = 12          // 消息头长度
	MsgBodySizeMax = 1024 * 1024 // 消息体上限
)

type MessageHead struct {
	ProtoId  pb.ProtocolId // 协议id
	PlayerId uint32        // 玩家id
	Length   uint32        // 消息体长度（不含头）
}

func (r *MessageHead) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, r.ProtoId); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, r.PlayerId); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, r.Length); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *MessageHead) Decode(data []byte) error {
	if len(data) < MsgHeadSize {
		return errors.New("message header too short")
	}
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &r.ProtoId); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.LittleEndian, &r.PlayerId); err != nil {
		return err
	}
	if err := binary.Read(buf, binary.LittleEndian, &r.Length); err != nil {
		return err
	}
	if r.Length > MsgBodySizeMax {
		return errors.New("message body too long")
	}
	return nil
}

func NewMessageHead(data []byte) *MessageHead {
	head := new(MessageHead)
	if err := head.Decode(data); err != nil {
		return nil
	}
	return head
}

type Message struct {
	Head *MessageHead
	Body []byte
}

func (r *Message) Bytes() []byte {
	if r.Head == nil || r.Body == nil {
		return nil // 关键字段缺失
	}
	headBytes, err := r.Head.Encode()
	if err != nil || len(headBytes) != MsgHeadSize {
		return nil // 头编码失败或长度异常
	}
	if uint32(len(r.Body)) != r.Head.Length {
		return nil // 头声明的长度与实际体长度不符
	}

	fullMsg := make([]byte, 0, MsgHeadSize+len(r.Body))
	fullMsg = append(fullMsg, headBytes...)
	fullMsg = append(fullMsg, r.Body...)
	return fullMsg
}

func NewMsg(head *MessageHead, body []byte) *Message {
	return &Message{
		Head: head,
		Body: body,
	}
}

func NewRpcMsg(msg proto.Message) *Message {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil
	}
	return NewMsg(&MessageHead{
		ProtoId: pb.ProtocolId_Hello,
		Length:  uint32(len(data)),
	}, data)
}

func NewCSMsg(protoId pb.ProtocolId, msg proto.Message) *Message {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil
	}
	return NewMsg(&MessageHead{
		ProtoId: protoId,
		Length:  uint32(len(data)),
	}, data)
}
