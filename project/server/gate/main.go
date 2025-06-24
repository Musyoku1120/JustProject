package main

import (
	"os"
	"path/filepath"
	"server/frame"
)

func main() {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/gate/global.yml")

	frame.InitConfig(configPath)
	frame.InitBase()
	frame.InitRPC()

	frame.WaitForExit()
}
