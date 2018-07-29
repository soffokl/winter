package main

import (
	"flag"

	"github.com/soffokl/winter/server"
)

func main() {
	telnet := flag.String("telnet", ":8080", "port for telnet connections")
	websocket := flag.String("websocket", ":8888", "port for websocket connections")

	flag.Parse()

	go server.NewServer("websocket", 10, 30).ListenAndServe(*websocket)
	server.NewServer("telnet", 10, 30).ListenAndServe(*telnet)
}
