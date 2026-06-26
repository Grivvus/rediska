package config

import (
	"fmt"
	"math/rand/v2"
)

type RoleType int

const (
	MasterRole RoleType = iota + 1
	ReplicaRole
)

func (r RoleType) String() string {
	switch r {
	case MasterRole:
		return "master"
	case ReplicaRole:
		return "slave"
	default:
		panic("Unkown role type")
	}
}

type RedisConfig struct {
	RdbDir           string
	RdbFilename      string
	Port             string
	Role             RoleType
	ConnectedSlaves  int
	MasterHost       string
	MasterPort       string
	MasterReplOffset int
	MasterReplid     [40]byte
}

func Default() *RedisConfig {
	return &RedisConfig{
		Port:         "6379",
		Role:         MasterRole,
		MasterReplid: generateReplid(),
	}
}

func (r RedisConfig) GetInfo() string {
	res := fmt.Sprintf(
		"role:%v\r\nmaster_repl_offset:%v\r\nmaster_replid:%s",
		r.Role.String(), r.MasterReplOffset, r.MasterReplid,
	)
	return res
}

func generateReplid() [40]byte {
	const characters = "abcdef1234567890"
	replid := [40]byte{}
	for i := range 40 {
		replid[i] = characters[rand.Int()%len(characters)]
	}
	return replid
}
