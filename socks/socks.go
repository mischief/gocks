package socks

import (
	"net"
	"log"
)

const (
	SocksV4 = iota
	SocksV5
)

type SocksConn interface {
	Setup(c net.Conn) error
	Io() error
}

type SocksServer struct {
	ver int
	ln net.TCPListener
	pending, complete chan net.Conn
}

func NewSocksServer(ver, handlers int) *SocksServer {
	if ver != SocksV4 {
		panic("Only Socks V4 supported")
	}
	
	sv := &SocksServer{ver: ver, pending: make(chan net.Conn),
		complete: make(chan net.Conn)}
	
	for i := 0; i < handlers; i++ {
		go sv.handleSocks(sv.pending, sv.complete)
	}

	go sv.handleComplete(sv.complete)
			
	return sv
}

func (sv *SocksServer) Listen(addr string) error {
	lAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}

	ln, err := net.ListenTCP("tcp", lAddr)
	if err != nil {
		return err
	}

	sv.ln = *ln
	
	log.Println(addr + " listening")

	return nil
}

func (sv *SocksServer) Loop() error {
	dead := 0

	for dead < 10 {
		conn, err := sv.ln.AcceptTCP()
		if err != nil {
			log.Println(err)
			dead++
		}

		sv.pending <- conn
	}
	
	return nil
}

func (sv *SocksServer) LoopOnce() error {
	return nil
}	

func (sv *SocksServer) Shutdown() {
	sv.ln.Close()
}

func (sv *SocksServer) handleSocks(in <-chan net.Conn, out chan<- net.Conn) {

	for conn := range in {
		var s SocksConn
		if sv.ver == SocksV4 {
			s = &Socks4Conn{}
		} else {
			panic("Only Socks V4 supported")
		}

		// setup connection via socks4 protocol
		if err := s.Setup(conn); err != nil {
			log.Println("Setup() failed: " + err.Error())
			return
		}

		if err := s.Io(); err != nil {
			log.Println("Io() failed: " + err.Error())
			return
		}

		out <- conn
	}
}

func (sv *SocksServer) handleComplete(in <-chan net.Conn) {
	for conn := range in {
		conn.Close()
	}
}
