package main

import (
	"os"
	"path/filepath"
	"server/frame"
)

func main() {
	frame.LogInfo("start gate")

	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/gate/global.yml")

	frame.InitConfig(configPath)
	frame.Init()

	frame.WaitForExit()
}
