package raft

import (
	"net"
	"sync"
)

type Server interface {
	// Call makes an RPC using the provided service method
	Call(id int, service string, args interface{}, res interface{}) error
	ConnectToPeer(peerId int, addr net.Addr) error
	Serve()
	GetListenAddr() net.Addr
}

type server struct {
	mu      sync.Mutex
	cm      *CnsModule
	enabled map[interface{}]bool
	ready   <-chan interface{}
	quit    chan interface{}
	wg      sync.WaitGroup
}

func (s *server) GetListenAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (s *server) Serve() {
	//TODO implement me
	panic("implement me")
}

func (s *server) ConnectToPeer(peerId int, addr net.Addr) error {
	//TODO implement me
	panic("implement me")
}

func (s *server) Call(id int, service string, args interface{}, res interface{}) error {
	//TODO implement me
	panic("implement me")
}

func NewServer(serverID int, peerIds []int, ready <-chan interface{}) Server {
	return &server{}
}
