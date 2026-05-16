//go:build !windows

package ipc

import (
	"net"
	"os"
	"strconv"
	"time"
)

func listen(address string) (net.Listener, error) {
	_ = os.Remove(address)
	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(address, 0600)
	return listener, nil
}

func dial(address string) (net.Conn, error) {
	return net.Dial("unix", address)
}

func dialTimeout(address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", address, timeout)
}

func userID() string {
	return strconv.Itoa(os.Getuid())
}
