package ipc

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const EnvSocket = "SCORE_SOCKET"

func DefaultAddress() string {
	if value := os.Getenv(EnvSocket); value != "" {
		return value
	}
	if runtime.GOOS == "windows" {
		return `\\.\pipe\score-daemon`
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "score.sock")
	}
	return filepath.Join(os.TempDir(), "score-"+userID()+".sock")
}

func ListenDefault() (net.Listener, string, error) {
	address := DefaultAddress()
	listener, err := listen(address)
	if err != nil {
		return nil, address, err
	}
	return listener, address, nil
}

func DialDefault() (net.Conn, string, error) {
	address := DefaultAddress()
	conn, err := dial(address)
	return conn, address, err
}

func DialDefaultTimeout(timeout time.Duration) (net.Conn, string, error) {
	address := DefaultAddress()
	conn, err := dialTimeout(address, timeout)
	return conn, address, err
}
