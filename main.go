package main

import "sync"

// LogEntry represents a log entry.
// It holds information about the term when the entry was received by the leader
// it contains command for the state machine
type LogEntry struct{}

// CnsModule is the consensus module on a server receives commands from clients and adds them to its log.
// It communicates with the consensus modules on other servers to ensure that every log eventually
// contains the same requests in the same order, even if some servers fail.
// The fields in this struct are defined in Fig.2 of the Raft Paper
type CnsModule struct{
	// persistent State Of all servers
	CurrentTerm int
	VotedFor int
	Log []LogEntry

// mu is a lock for synchronized access to this struct
	mu sync.Mutex
	// Me is the ID of this specific Node 
	Me int
   // peerIds 
}

func (cm * CnsModule)
