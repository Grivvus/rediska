package main

import (
	"fmt"
	"math/rand"
)

type redisConfig struct {
	RdbDir           string
	RdbFilename      string
	Port             string
	Role             RoleType
	ConnectedSlaves  int
	MasterHost       string
	MasterPort       string
	MasterReplid     [40]byte
	MasterReplOffset int
}

type RoleType = int

const (
	Master RoleType = iota
	Replica
)

var config *redisConfig = nil

func GetSettings() *redisConfig {
	if config == nil {
		config = new(redisConfig)
		config.Port = "6379"
		config.Role = Master
		generateReplid()
	}
	return config
}

func GetInfo() string {
	res := EncodeString(fmt.Sprintf("role:%v\r\nmaster_repl_offset:%v\r\nmaster_replid:%s", roleToString(config.Role), config.MasterReplOffset, config.MasterReplid))
	return res
}

func generateReplid() {
	const characters = "abcdef1234567890"
	if config == nil {
		panic("redisConfig is nil")
	}
	replid := [40]byte{}
	for i := range 40 {
		replid[i] = characters[rand.Int()%len(characters)]
	}
	config.MasterReplid = replid
}

func roleToString(role RoleType) string {
	switch role {
	case Master:
		return "master"
	case Replica:
		return "slave"
	default:
		panic("Unkown role type")
	}
}
