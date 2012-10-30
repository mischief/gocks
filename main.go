package main

import (
	"encoding/binary"
	"flag"
	"io"
	"log"
	"net"
	"bufio"
	"fmt"
)

var listenAddr	*string = flag.String("l", "127.0.0.1:8080", "listening address")
var goCount	*int = flag.Int("g", 1, "number of goroutine handlers")

type SocksConn struct {
	Con	*net.TCPConn
	ConRW	*bufio.Reader

	RemoteIP net.IP
	RemotePort uint16
	Remote	*net.TCPConn
	RemoteRW *bufio.Reader

	Quit	chan bool
}

func newSocksConn(con *net.TCPConn) *SocksConn {
	s := new(SocksConn)

	s.Con = con
	s.Quit = make(chan bool)

	return s
}

func handleSocks(in <-chan *net.TCPConn, out chan<- *net.TCPConn) {

	for conn := range in {

		log.Printf("New client: %s", conn.RemoteAddr().String())

		s := newSocksConn(conn)
		proxySocks(s)
		out <- conn
	}
}

func proxySocks(s *SocksConn) {

	// setup connection via socks4 protocol
	if s.Setup() != 0 {
		return
	}

	if s.Dial() != 0 {
		return
	}

	s.Loop()

}

// sets up connection according to socks4
func (c *SocksConn) Setup() int {

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

	c.RemotePort = dstport

	// ip
	if err = binary.Read(c.ConRW, binary.BigEndian, &dstip); err != nil {
		goto error
	}

	ip = net.IPv4(dstip[0], dstip[1], dstip[2], dstip[3])
	c.RemoteIP = ip

	// user
	if user, err = c.ConRW.ReadString(0); err != nil {
		goto error
	}

	log.Printf("Ver: %X Cmd: %X Port: %d IP: %v User: %s", vn, cd, dstport, ip, user)

	// Reply
	c.Con.Write([]byte{0, 90, 0, 0, 0, 0, 0, 0})

	return 0

error:
	if err != nil {
		log.Println(err)
	}

	return 1
}

// Dial the remote server.
func (s *SocksConn) Dial() int {

	remote := fmt.Sprintf("%s:%d", s.RemoteIP.String(), s.RemotePort)

	rAddr, err := net.ResolveTCPAddr("tcp", remote)
	if err != nil {
		log.Printf("Dial(): %s", err.Error())
		return 1
	}

	rem, err := net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		log.Printf("Dial(): %s", err.Error())
		return 1
	}

	s.Remote = rem

	log.Println("Successfully connected to " + remote)

	// FINISH ME

	return 0
}

// loops, sending data between remote ends of socks4 proxy.
func (s *SocksConn) Loop() {
	go io.Copy(s.Con, s.Remote)
	io.Copy(s.Remote, s.Con)
}

func handleComplete(in <-chan *net.TCPConn) {
	for conn := range in {
		conn.Close()
	}
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

	pending, complete := make(chan *net.TCPConn), make(chan *net.TCPConn)

	for i := 0; i < *goCount; i++ {
		go handleSocks(pending, complete)
	}

	go handleComplete(complete)

	defer ln.Close()

	dead := 0

	for dead < 10 {
		conn, err := ln.AcceptTCP()
		if err != nil {
			log.Println(err)
			dead++
		}

		pending <- conn
	}

}
