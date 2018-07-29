package connection

import (
	"io"
)

type Telnet struct{}

func (t *Telnet) ListenAndServe(address string) <-chan io.ReadWriteCloser {
	return listenAndServe(address, nil)
}
