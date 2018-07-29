package server

import (
	"io"
	"time"

	"github.com/soffokl/winter/server/connection"
)

const (
	timeForStep = time.Second * 3
)

type communicationChannel interface {
	ListenAndServe(address string) <-chan io.ReadWriteCloser
}

type server struct {
	w, h int
	conn communicationChannel
}

func (s server) ListenAndServe(address string) {
	for r := range s.conn.ListenAndServe(address) {
		go newSession(r, s.h, s.w, timeForStep)
	}
}

// NewServer provides server instance for the "Winter is coming" game.
// Telnet and Websocket protocols are supported at the moment.
func NewServer(serverType string, height, width int) server {
	s := server{h: height, w: width}
	switch serverType {
	case "websocket":
		s.conn = &connection.WebSocket{}
	case "telnet":
		s.conn = &connection.Telnet{}
	default:
		panic("unknown server type specified, only websocket and telnet supported now")
	}
	return s
}
