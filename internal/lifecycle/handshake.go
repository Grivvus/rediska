package lifecycle

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/codecrafters-io/redis-starter-go/internal/codec"
	"github.com/codecrafters-io/redis-starter-go/internal/config"
	"github.com/codecrafters-io/redis-starter-go/internal/storage"
)

func Handshake(
	ctx context.Context, cfg config.RedisConfig, st *storage.Storage,
) error {
	conn, err := getMasterConnection(cfg)
	if err != nil {
		return err
	}
	buffer := make([]byte, 100)
	ping(conn)
	_, err = conn.Read(buffer)
	log.Println(string(buffer))
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	replconfPort(cfg, conn)
	_, err = conn.Read(buffer)
	log.Println(string(buffer))
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	replconfCapa(conn)
	_, err = conn.Read(buffer)
	log.Println(string(buffer))
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	psync(conn)
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("can't read from master: %w", err)
	}
	go func() {
		handleConnection(ctx, cfg, conn, st, []net.Conn{} /*known replicas*/)
	}()
	return nil
}

func ping(conn net.Conn) {

	_, _ = conn.Write(codec.EncodeArray([]string{"PING"}))
}

func getMasterConnection(cfg config.RedisConfig) (net.Conn, error) {
	masterConn, err := net.Dial("tcp", cfg.MasterHost+":"+cfg.MasterPort)
	if err != nil {
		return nil, fmt.Errorf("can't connect to master: %w", err)
	}
	return masterConn, nil
}

func replconfPort(cfg config.RedisConfig, conn net.Conn) {
	_, _ = conn.Write([]byte(codec.EncodeArray([]string{"REPLCONF", "listening-port", cfg.Port})))
}

func replconfCapa(conn net.Conn) {
	_, _ = conn.Write([]byte(codec.EncodeArray([]string{"REPLCONF", "capa", "psync2"})))
}

func psync(conn net.Conn) {
	_, _ = conn.Write([]byte(codec.EncodeArray([]string{"PSYNC", "?", "-1"})))
}
