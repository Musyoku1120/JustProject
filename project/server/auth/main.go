package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/labstack/echo"
	"net/http"
	"os"
	"path/filepath"
	"server/frame"
	"time"
)

type LoginReq struct {
	Account string `json:"account"`
}
type EnterReq struct {
	Session string `json:"session"`
}

var (
	CacheAccount map[string]int32
	CacheSession map[string]int32
)

func main() {
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "/server/auth/global.yml")

	frame.InitConfig(configPath)
	frame.InitBase()

	frame.LogInfo("start auth")
	CacheAccount = make(map[string]int32)
	CacheSession = make(map[string]int32)

	go Start()

	frame.WaitForExit()
}

func Start() {
	eo := echo.New()
	eoGroup := eo.Group("/auth")
	eoGroup.POST("/login", httpLogin)
	eoGroup.POST("/enter", httpEnter)
	if err := eo.Start("127.0.0.1:1120"); err != nil {
		panic(err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cancel()
		_ = eo.Shutdown(ctx)
	}()
}

func MD5Bytes(s []byte) string {
	md5Ctx := md5.New()
	md5Ctx.Write(s)
	cipherStr := md5Ctx.Sum(nil)
	return hex.EncodeToString(cipherStr)
}

func httpResponse(ec echo.Context, res interface{}) error {
	ec.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return ec.JSON(http.StatusOK, res)
}

func httpLogin(ec echo.Context) error {
	req := &LoginReq{}
	if err := ec.Bind(req); err != nil {
		return err
	}

	roleId, ok := CacheAccount[req.Account]
	if !ok {
		roleId = int32(len(CacheAccount) + 1)
		CacheAccount[req.Account] = roleId
	}

	session := MD5Bytes([]byte(fmt.Sprintf("%v%v", req.Account, frame.TimeStamp)))
	CacheSession[session] = roleId
	return httpResponse(ec, session)
}

func httpEnter(ec echo.Context) error {
	req := &EnterReq{}
	if err := ec.Bind(req); err != nil {
		return err
	}

	roleId, ok := CacheSession[req.Session]
	if !ok {
		return httpResponse(ec, nil)
	}
	return httpResponse(ec, roleId)
}
