package main

import "fmt"

type redisConfig struct {
	RdbDir           string
	RdbFilename      string
	Port             string
	Role             string
	ConnectedSlaves  int
	MasterHost       string
	MasterPort       int
	MasterReplid     []byte
	MasterReplOffset int
}

var config *redisConfig = nil

func GetSettings() *redisConfig {
	if config == nil {
		config = new(redisConfig)
		config.Port = "6379"
		config.Role = "master"
	}
	return config
}

func GetInfo() string {
	return Encode(fmt.Sprintf("role:%v", config.Role))
}
