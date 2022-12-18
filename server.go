package raft

import (
	"net"
	"sync"
)

type IServer interface {
	// Call makes an RPC using the provided service method
	Call(id int, service string, args interface{}, res interface{}) error
	ConnectToPeer(peerId int, addr net.Addr) error
	Serve()
	GetListenAddr() net.Addr
}

type Server struct {
	mu sync.Mutex
	//cm      *CnsModule
	enabled map[interface{}]bool
	ready   <-chan interface{}
	quit    chan interface{}
	wg      sync.WaitGroup
	CnsModule
}

func (s *Server) GetListenAddr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Serve() {
	//TODO implement me
	panic("implement me")
}

func (s *Server) ConnectToPeer(peerId int, addr net.Addr) error {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Call(id int, service string, args interface{}, res interface{}) error {
	//TODO implement me
	panic("implement me")
}

func NewServer(serverID int, peerIds []int, ready <-chan interface{}) *Server {
	return &Server{}
}
