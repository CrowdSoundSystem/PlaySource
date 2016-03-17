package systemd

import (
	"errors"
	"net"
	"os"
)

var ErrNoSDNotifySocket = errors.New("no sd_notify socket")

func sdNotify(state string) error {
	socketAddr := &net.UnixAddr{
		Name: os.Getenv("NOTIFY_SOCKET"),
		Net:  "unixgram",
	}

	if socketAddr.Name == "" {
		return ErrNoSDNotifySocket
	}

	conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	return err
}

func Ready() error {
	return sdNotify("READY=1")
}

func Stopping() error {
	return sdNotify("STOPPING=1")
}
