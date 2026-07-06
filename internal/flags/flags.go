package flags

import (
	"flag"
	"strings"
)

type ParseResult struct {
	RdbDir      string
	RdbFilename string
	Port        string
	Master      *MasterInfo
}

type MasterInfo struct {
	Host string
	Port string
}

func Parse() ParseResult {
	port := flag.String("port", "6379", "set the port for the redis server")
	rdbDir := flag.String("dir", "", "directory with rdb files")
	rdbFile := flag.String("dbfilename", "", "rdb filename")
	masterInfo := flag.String("replicaof", "", "information about master node provided to the replica")
	var mInfo *MasterInfo
	if *masterInfo != "" {
		splited := strings.Split(*masterInfo, " ")
		mInfo = &MasterInfo{
			Host: splited[0],
			Port: splited[1],
		}
	}
	return ParseResult{
		RdbDir:      *rdbDir,
		Port:        *port,
		RdbFilename: *rdbFile,
		Master:      mInfo,
	}
}
