package main

type redisConfig struct {
    RdbDir string
    RdbFilename string
    Port string
}

var config *redisConfig = nil

func GetSettings() *redisConfig{
    if config == nil {
        config = new(redisConfig)
        config.Port = "6379"
    }
    return config
}
