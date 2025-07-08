package main

import (
	"encoding/hex"
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

	http.HandleFunc("/proxy", func(writer http.ResponseWriter, request *http.Request) {
		frame.StartProxy(writer, request)
	})
	go func() {
		ws := &http.Server{Addr: ":1000"}
		defer func() {
			_ = ws.Close()
		}()
		if err := ws.ListenAndServe(); err != nil {
			frame.LogError("Start Listen And Serve err:%v", err)
			return
		}
	}()

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

	fmt.Println("login:", hex.EncodeToString(login.Bytes()))
	fmt.Println("common:", hex.EncodeToString(common.Bytes()))
}
