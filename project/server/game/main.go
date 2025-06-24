package main

import (
	"os"
	"path/filepath"
	"server/frame"
	"server/protocol/generate/pb"
	"server/server/tool"
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
	frame.DefaultMsgHandler.AddHandlers(server.CSHandlerMap) // CSHandlerObj Need Register
	server.CSHandlerObj.HandlerLogin = func(req *pb.LoginC2S) error {
		frame.LogInfo("handler login")
		return nil
	}
	server.CSHandlerObj.HandlerCommon = func(req *pb.CommonC2S) error {
		frame.LogInfo("handler common")
		return nil
	}
}
