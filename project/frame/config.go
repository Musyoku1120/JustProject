package frame

import (
	"github.com/stretchr/testify/assert/yaml"
	"os"
)

var (
	ServerTypeAuth = "auth"
	ServerTypeGate = "gate"
	ServerTypeGame = "game"

	validServerTypes = map[string]struct{}{
		ServerTypeAuth: {},
		ServerTypeGate: {},
		ServerTypeGame: {},
	}
)

type ConfigGlobal struct {
	UniqueId   int32    `yaml:"UniqueId"`
	LogPath    string   `yaml:"LogPath"`
	ServerType string   `yaml:"ServerType"`
	Address    string   `yaml:"Address"`
	ServerAddr []string `yaml:"ServerAddr"`
}

func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		LogError("read file filed path:%v err:%v", path, err)
		return nil, err
	}
	return data, nil
}

func InitConfig(globalPath string) {
	fileData, err := ReadFile(globalPath)
	if err != nil {
		LogError("read file filed path:%v err:%v", fileData, err)
		return
	}

	Global = &ConfigGlobal{}
	err = yaml.Unmarshal(fileData, Global)
	if err != nil {
		LogError("parse file filed path:%v err:%v", fileData, err)
		return
	}

	if _, ok := validServerTypes[Global.ServerType]; !ok {
		LogError("invalid server type:%v", Global.ServerType)
		return
	}
}
