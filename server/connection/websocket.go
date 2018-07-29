package connection

import (
	"bytes"
	"io"
	"log"
	"net"

	"github.com/gobwas/ws"
)

type WebSocket struct{}

type webSocketWrapper struct {
	c net.Conn
	h ws.Header
}

func newWebSocketWrapper(c net.Conn) io.ReadWriteCloser {
	_, err := ws.Upgrade(c)
	if err != nil {
		log.Println("failed to upgrade connection:", err)
	}
	return &webSocketWrapper{c: c}
}

func (w *webSocketWrapper) Read(b []byte) (int, error) {
	header, err := ws.ReadHeader(w.c)
	w.h = header
	if err != nil {
		return 0, err
	}

	n, err := w.c.Read(b[:w.h.Length])
	if err != nil {
		return 0, err
	}

	if w.h.Masked {
		ws.Cipher(b[:n], w.h.Mask, 0)
	}

	if n < len(b) {
		b[n] = '\n'
		n++
	}

	if w.h.OpCode == ws.OpClose {
		return n, io.EOF
	}
	return n, err
}

func (w *webSocketWrapper) Write(b []byte) (int, error) {
	output := bytes.TrimSpace(b)
	w.h.Masked = false
	w.h.Length = int64(len(output))
	if err := ws.WriteHeader(w.c, w.h); err != nil {
		return 0, err
	}
	return w.c.Write(output)
}

func (w *webSocketWrapper) Close() error {
	return w.c.Close()
}

func (w *WebSocket) ListenAndServe(address string) <-chan io.ReadWriteCloser {
	return listenAndServe(address, newWebSocketWrapper)
}
