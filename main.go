package main

import (
	"encoding/binary"
	"flag"
	"io"
	"log"
	"net"
	"bufio"

//	"socks/socks4"
)

var listenAddr *string = flag.String("l", "127.0.0.1:8080", "listening address")

type SocksConn struct {
	Con	*net.TCPConn
	ConRW	*bufio.Reader
	Remote	*net.TCPConn
	RemoteRW *bufio.Reader
	Quit	chan bool
}

func handleConn(c *net.TCPConn) {

	a := c.RemoteAddr()
	log.Printf("Handling %s", a.String())

	defer c.Close()

	// Allocate new structure for client
	n := &SocksConn{Con: c, Quit: make(chan bool)}

	// setup connection via socks4 protocol
	n.Setup()
	n.Loop()

}

// sets up connection according to socks4
func (c *SocksConn) Setup() {

	var err error

	var vn, cd uint8
	var dstport uint16
	var dstip [4]byte
	
	var ip net.IP
	var user string

	c.ConRW = bufio.NewReader(c.Con)

	// version
	if err = binary.Read(c.ConRW, binary.BigEndian, &vn); err != nil {
		goto error
	}

	// command
	if err = binary.Read(c.ConRW, binary.BigEndian, &cd); err != nil {
		goto error
	}

	// port
	if err = binary.Read(c.ConRW, binary.BigEndian, &dstport); err != nil {
		goto error
	}

	// ip
	if err = binary.Read(c.ConRW, binary.BigEndian, &dstip); err != nil {
		goto error
	}

	ip = net.IPv4(dstip[0], dstip[1], dstip[2], dstip[3])

	// user
	if user, err = c.ConRW.ReadString(0); err != nil {
		goto error
	}

	log.Printf("Ver: %X Cmd: %X Port: %d IP: %v User: %s", vn, cd, dstport, ip, user)

	// Reply
	c.Con.Write([]byte{0, 90, 0, 0, 0, 0, 0, 0})

error:
	if err != nil {
		log.Println(err)
	}
}

// loops, sending data between remote ends of socks4 proxy.
func (c *SocksConn) Loop() {
	io.Copy(c.Con, c.Con)
}

func main() {
	lAddr, err := net.ResolveTCPAddr("tcp", *listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenTCP("tcp", lAddr)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Listening on " + *listenAddr)

	defer ln.Close()

	dead := 0

	for dead < 10 {
		conn, err := ln.AcceptTCP()
		if err != nil {
			log.Println(err)
			dead++
		}

		go handleConn(conn)

	}

}
