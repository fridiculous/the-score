//go:build !windows

package ipc

import (
	"net"
	"os"
	"strconv"
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

func userID() string {
	return strconv.Itoa(os.Getuid())
}
