package main

import (
	"os"
	"path/filepath"
	"server/frame"
)

func main() {
	frame.LogInfo("start game")

	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/game/global.yml")

	frame.InitConfig(configPath)
	frame.Init()

	frame.WaitForExit()
}
