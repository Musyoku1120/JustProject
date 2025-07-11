package main

import (
	"os"
	"path/filepath"
	"server/frame"
	"server/protocol/generate"
	"server/protocol/generate/pb"
)

func main() {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/game/global.yml")

	frame.InitConfig(configPath)
	frame.InitBase()

	InitHandler()
	frame.InitRPC()

	frame.WaitForExit()
}

func InitHandler() {
	frame.DefaultMsgHandler.RegisterHandlers(generate.CSHandlerMap) // CSHandlerObj Need Register
	generate.CSHandlerObj.HandlerLogin = func(mq frame.IMsgQue, req *pb.LoginC2S) error {
		frame.LogInfo("handler login")
		mq.Send(frame.NewReplyMsg(req.RoleId, &pb.LoginS2C{
			Error: 0,
			Data:  "hello",
		}))
		return nil
	}
	generate.CSHandlerObj.HandlerCommon = func(mq frame.IMsgQue, req *pb.CommonC2S) error {
		frame.LogInfo("handler common")
		mq.Send(frame.NewReplyMsg(req.RoleId, &pb.CommonS2C{
			Error: 0,
		}))
		return nil
	}
}
