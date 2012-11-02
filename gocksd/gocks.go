package main

import (
	"flag"
	"gocks/socks"
)

var listenAddr *string = flag.String("l", "127.0.0.1:8080", "listening address")
var goCount *int = flag.Int("g", 1, "number of goroutine handlers")

func main() {
	flag.Parse()
	sv := socks.NewSocksServer(socks.SocksV4, 2)
	sv.Listen(*listenAddr)
	sv.Loop()
}
