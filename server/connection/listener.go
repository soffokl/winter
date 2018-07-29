package connection

import (
	"io"
	"log"
	"net"
)

func listenAndServe(address string, wrapper func(net.Conn) io.ReadWriteCloser) <-chan io.ReadWriteCloser {
	ch := make(chan io.ReadWriteCloser, 10)

	ln, err := net.Listen("tcp", address)
	if err != nil {
		panic(err) //TODO better error handing
	}

	go func() {
		defer ln.Close()
		defer close(ch)

		for {
			c, err := ln.Accept()
			if err != nil {
				log.Println(err)
				return
			}

			var conn io.ReadWriteCloser = c
			if wrapper != nil {
				conn = wrapper(c)
			}

			select {
			case ch <- conn:
			default:
			}
		}
	}()

	return ch
}
