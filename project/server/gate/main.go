package main

import (
	"net/http"
	"os"
	"path/filepath"
	"server/frame"
	"server/server/gate/serve"
)

func main() {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/gate/global.yml")

	frame.InitConfig(configPath)
	frame.InitBase()
	frame.InitRPC()

	http.HandleFunc("/proxy", func(writer http.ResponseWriter, request *http.Request) {
		serve.StartProxy(writer, request)
	})

	frame.WaitForExit()
}
