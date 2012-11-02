package socks

import (
	"net"
	"fmt"
	"io"
	"bufio"
	"encoding/binary"
	"errors"
	"log"
)

const (
	SOCKS4ID      = 4
	SOCKS4CONNECT = 1
	SOCKS4BIND    = 2
	
	// ok
	SOCKS4REQOK   = 90
	// rejected
	SOCKS4REQFAIL = 91
	// no identd
	SOCKS4NOIDENT = 92
	// ident failed
	SOCKS4IDENTFAIL = 93
)

type Socks4Conn struct {
	Client, Remote net.Conn
}

type Socks4Request struct {
	ver, command uint8
	dstport uint16
	dstip net.IP
	userid string
}

type Socks4Reply struct {
	unused1 byte
	reply byte
	unused2 uint16
	unused3 uint32
}

func (s *Socks4Conn) Setup(c net.Conn) error {

	var err error
	var tempip [4]uint8
	
	s.log("connected")

	var req Socks4Request
	var rep Socks4Reply
	var v uint8

	cr := bufio.NewReader(s.Client)
	cw := bufio.NewWriter(s.Client)

	// version
	if err = binary.Read(cr, binary.BigEndian, v); err != nil {
		goto error
	}

	if req.ver != SOCKS4ID {
		err = errors.New("Setup(): invalid protocol version: " + fmt.Sprintf("%d", req.ver))
		goto error
	}

	// command
	if err = binary.Read(cr, binary.BigEndian, &req.command); err != nil {
		goto error
	}
	
	if req.command != SOCKS4CONNECT {
		err = errors.New("Setup(): invalid protocol command " + fmt.Sprintf("%d", req.command))
		goto error
	}

	// port
	if err = binary.Read(cr, binary.BigEndian, &req.dstport); err != nil {
		goto error
	}

	// ip
	if err = binary.Read(cr, binary.BigEndian, &tempip); err != nil {
		goto error
	}

	// user
	if req.userid, err = cr.ReadString(0); err != nil {
		goto error
	}

	req.dstip = net.IPv4(tempip[0], tempip[1], tempip[2], tempip[3])

	s.log(fmt.Sprintf("Ver %X Cmd: %X Port: %d IP: %v User: %s",
		req.ver, req.command, req.dstport, req.dstip, req.userid))

	// Reply
	rep.reply = SOCKS4REQOK
	if err = binary.Write(cw, binary.BigEndian, &rep); err != nil {
		goto error
	}

	return nil
	
error:
	rep.reply = SOCKS4REQFAIL
	if err2 := binary.Write(cw, binary.BigEndian, rep); err2 != nil {
		s.log("Error error: " + err2.Error())
	}

	return err
}

// loops, sending data between remote ends of socks4 proxy.
func (s *Socks4Conn) Io() error {

	sync := make(chan error, 2)
	
	if err := s.dial(); err != nil {
		return err
	}

	// remote -> client
	go s.netcopy(s.Remote, s.Client, sync)

	// client -> remote
	go s.netcopy(s.Client, s.Remote, sync)

	if err := <-sync; err != nil {
		s.Remote.Close()
		<- sync
		return err
	}
	
	if err := <-sync; err != nil {
		return err
	}

	return nil
}

func (s *Socks4Conn) log(msg string) {
	if s.Client != nil {
		log.Printf("%s: %s", s.Client.RemoteAddr().String(), msg)
	} else {
		log.Println(msg)
	}
}

// Dial the remote server.
func (s *Socks4Conn) dial() error {

	remote := s.Remote.RemoteAddr().String()

	rAddr, err := net.ResolveTCPAddr("tcp", remote)
	if err != nil {
		return err
	}

	rem, err := net.DialTCP("tcp", nil, rAddr)
	if err != nil {
		return err
	}

	s.Remote = rem

	s.log("Successfully connected to " + remote)

	return nil
}

func (s *Socks4Conn) netcopy(to net.Conn, from net.Conn, quit chan<- error) {

	if _, err := io.Copy(to, from); err != nil {
		quit <- err
		return

		//~ if err := from.Close(); err != nil {
			//~ s.SocksLog("Close(): " + err.Error())
		//~ }
	}

	quit <- nil
}
