package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
)

var listenAddr *string = flag.String("l", "127.0.0.1:8080", "listening address")
var goCount *int = flag.Int("g", 1, "number of goroutine handlers")

type SocksConn struct {
	Con   *net.TCPConn
	ConRW *bufio.Reader

	RemoteIP   net.IP
	RemotePort uint16
	Remote     *net.TCPConn
	RemoteRW   *bufio.Reader
}

func newSocksConn(con *net.TCPConn) *SocksConn {
	s := new(SocksConn)
	s.Con = con

	return s
}

func (s *SocksConn) GetClientAddr() string {
	if s.Con != nil {
		return s.Con.RemoteAddr().String()
	}

	return "<nil>"
}

func (s *SocksConn) GetRemoteAddr() string {
	if s.Remote != nil {
		return s.Remote.RemoteAddr().String()
	}

	return ""
}

func (s *SocksConn) SocksLog(msg string) {
	if s.GetRemoteAddr() == "" {
		log.Printf("%s: %s", s.GetClientAddr(), msg)
	} else {
		log.Printf("%s -> %s: %s", s.GetClientAddr(), s.GetRemoteAddr(), msg)
	}
}

func (s *SocksConn) ProxySocks() {

	// setup connection via socks4 protocol
	if s.Setup() != 0 {
		s.SocksLog("Setup() failed")
		return
	}

	if s.Dial() != 0 {
		s.SocksLog("Dial() failed")
		return
	}

	s.Loop()
	
	s.SocksLog("done")
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

	// ip
	if err = binary.Read(c.ConRW, binary.BigEndian, &dstip); err != nil {
		goto error
	}

	// user
	if user, err = c.ConRW.ReadString(0); err != nil {
		goto error
	}

	ip = net.IPv4(dstip[0], dstip[1], dstip[2], dstip[3])
	c.RemoteIP = ip
	c.RemotePort = dstport

	c.SocksLog(fmt.Sprintf("Ver %X Cmd: %X Port: %d IP: %v User: %s", vn, cd, dstport, ip, user))

	if vn != 4 || cd != 1 {
		goto error
	}

	// Reply
	c.Con.Write([]byte{0, 90, 0, 0, 0, 0, 0, 0})

	return 0

error:
	c.Con.Write([]byte{0, 91, 0, 0, 0, 0, 0, 0})

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
		s.SocksLog(fmt.Sprintf("ResolveTCPAddr(): %s", err.Error()))
		return 1
	}

	rem, err := net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		s.SocksLog(fmt.Sprintf("DialTCP(): %s", err.Error()))
		return 1
	}

	s.Remote = rem

	s.SocksLog("Successfully connected to " + remote)

	// FINISH ME

	return 0
}

func (s *SocksConn) netcopy(to *net.TCPConn, from *net.TCPConn, quit chan<- bool) {

	if _, err := io.Copy(to, from); err != nil {
		s.SocksLog("Copy(): " + err.Error())

		if err := from.Close(); err != nil {
			s.SocksLog("Close(): " + err.Error())
		}
	}

	quit <- true
}

// loops, sending data between remote ends of socks4 proxy.
func (s *SocksConn) Loop() {
	sync := make(chan bool, 2)

	// remote -> client
	go s.netcopy(s.Remote, s.Con, sync)

	// client -> remote
	go s.netcopy(s.Con, s.Remote, sync)

	<-sync
	<-sync
}

func handleSocks(in <-chan *net.TCPConn, out chan<- *net.TCPConn) {

	for conn := range in {
		s := newSocksConn(conn)

		s.SocksLog("connected")

		s.ProxySocks()
		out <- conn
	}
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

	log.Println(*listenAddr + " listening")

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
