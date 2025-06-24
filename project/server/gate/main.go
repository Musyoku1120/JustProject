package main

import (
	"os"
	"path/filepath"
	"server/frame"
	"server/protocol/generate/pb"
	"time"
)

func main() {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/gate/global.yml")

	frame.InitConfig(configPath)
	frame.InitBase()
	frame.InitRPC()

	TestMsg()

	frame.WaitForExit()
}

func TestMsg() {

	time.Sleep(3 * time.Second)

	mq101 := frame.GetProtoServiceMq(101)
	mq102 := frame.GetProtoServiceMq(102)

	ticker := time.NewTicker(time.Second * 3)
	for {
		select {
		case <-ticker.C:
			mq101.Send(frame.NewProtoMsg(pb.ProtocolId_Login, &pb.LoginC2S{
				RoleId: 123,
			}))
			mq102.Send(frame.NewProtoMsg(pb.ProtocolId_Common, &pb.CommonC2S{
				RoleId: 456,
			}))
		}
	}
}
