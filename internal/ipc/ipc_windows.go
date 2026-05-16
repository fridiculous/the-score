//go:build windows

package ipc

import (
	"net"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
)

func listen(address string) (net.Listener, error) {
	return winio.ListenPipe(address, nil)
}

func dial(address string) (net.Conn, error) {
	return winio.DialPipe(address, nil)
}

func dialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(address, &timeout)
}

func userID() string {
	if value := os.Getenv("USERNAME"); value != "" {
		return value
	}
	return "user"
}
