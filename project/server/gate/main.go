package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"server/frame"
	"server/protocol/generate/pb"
)

func main() {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/gate/global.yml")

	frame.InitConfig(configPath)
	frame.InitBase()
	frame.InitRPC()

	_ = http.ListenAndServe("127.0.0.1:5000", nil)
	http.HandleFunc("/proxy", func(writer http.ResponseWriter, request *http.Request) {
		frame.StartProxy(writer, request)
	})

	Prt()
	frame.WaitForExit()
}

func Prt() {
	login := frame.NewProtoMsg(pb.ProtocolId_Login, 101, &pb.LoginC2S{
		RoleId: 101,
	})
	common := frame.NewProtoMsg(pb.ProtocolId_Common, 101, &pb.CommonC2S{
		RoleId: 101,
	})

	fmt.Println("login:", login.Bytes())
	fmt.Println("common:", common.Bytes())
}
