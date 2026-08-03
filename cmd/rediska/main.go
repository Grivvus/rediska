package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/flags"
	"github.com/codecrafters-io/redis-starter-go/internal/lifecycle"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, os.Kill, syscall.SIGTERM,
	)
	defer cancel()

	slog.Info("Rediska startup")
	providedFlags := flags.Parse()

	cfg := config.Default()
	cfg.WithFlags(providedFlags)

	st := storage.NewStorage(*cfg)

	if cfg.Role == config.ReplicaRole {
		go func() {
			err := lifecycle.Handshake(ctx, *cfg, st)
			if err != nil {
				slog.Error("error during handshake", "err", err)
			}
		}()
	}

	if cfg.RdbDir != "" || cfg.RdbFilename != "" {
		f, err := os.Open(cfg.RdbDir + cfg.RdbFilename)
		if err != nil {
			slog.Error("can't open rdb file", "err", err)
			return
		}
		rdb, err := storage.DecodeRDB(f)
		if err != nil {
			slog.Error("can't parse rdb file", "err", err)
			return
		}
		rdb.Apply(st)
	}

	err := lifecycle.Listen(ctx, *cfg, st)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	<-ctx.Done()
	log.Println("rediska shutdown")
}
