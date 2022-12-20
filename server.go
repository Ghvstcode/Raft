package raft

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
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
	mu      sync.Mutex
	me      int
	cm      *CnsModule
	enabled map[interface{}]bool
	ready   <-chan interface{}
	quit    chan interface{}
	wg      sync.WaitGroup
	peers   map[int]*rpc.Client
	//cm CnsModule
	peerIds   []int
	listener  net.Listener
	rpcServer *rpc.Server
	rpcProxy  interface{}
}

func (s *Server) GetListenAddr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Serve() {
	s.mu.Lock()
	//s.cm = NewConsensusModule(s.serverId, s.peerIds, s, s.ready)

	// Create a new RPC server and register a RPCProxy that forwards all methods
	// to n.cm
	s.rpcServer = rpc.NewServer()
	s.rpcProxy = &Proxy{cm: s.cm}
	s.rpcServer.RegisterName("CnsModule", s.rpcProxy)

	var err error
	s.listener, err = net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[Server-%v] is listening on port %s", s.me, s.listener.Addr())
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.listen()
	}()
}

func (s *Server) listen() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				log.Fatal("accept error:", err)
			}
		}
		s.wg.Add(1)
		go func() {
			s.rpcServer.ServeConn(conn)
			s.wg.Done()
		}()
	}
}

func (s *Server) ConnectToPeer(peerId int, addr net.Addr) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peers[peerId] == nil {
		client, err := rpc.Dial(addr.Network(), addr.String())
		if err != nil {
			return err
		}
		s.peers[peerId] = client
	}
	return nil
}

func (s *Server) Call(id int, service string, args interface{}, res interface{}) error {
	s.mu.Lock()
	peer := s.peers[id]
	s.mu.Unlock()

	// If this is called after shutdown (where client.Close is called), it will
	// return an error.
	if peer == nil {
		return fmt.Errorf("call client %d after it's closed", id)
	} else {
		return peer.Call(service, args, res)
	}
}

func (s *Server) DisconnectAllPeers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.peers {
		if s.peers[id] != nil {
			s.peers[id].Close()
			s.peers[id] = nil
		}
	}
}

func (s *Server) DisconnectPeer(peerId int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peers[peerId] != nil {
		err := s.peers[peerId].Close()
		s.peers[peerId] = nil
		return err
	}
	return nil
}

func NewServer(serverID int, peerIds []int, ready <-chan interface{}) *Server {
	s := new(Server)
	s.me = serverID
	s.peerIds = peerIds
	s.peers = make(map[int]*rpc.Client)
	s.ready = ready
	s.quit = make(chan interface{})
	return s
}
