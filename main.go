package main

import (
	"math/rand"
	"sync"
	"time"
)

// LogEntry represents a log entry.
// It holds information about the term when the entry was received by the leader
// it contains command for the state machine
type LogEntry struct{}

// Persistence dummy struct
type Persistence struct{}
type RftState int

const (
	Follower RftState = iota
	Candidate
	Leader
	Dead
)

func (s RftState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	case Dead:
		return "Dead"
	default:
		return "Unkown"
	}
}

// CnsModule is the consensus module on a server receives commands from clients and adds them to its log.
// It communicates with the consensus modules on other servers to ensure that every log eventually
// contains the same requests in the same order, even if some servers fail.
// The fields in this struct are defined in Fig.2 of the Raft Paper
type CnsModule struct {
	// persistent State Of all servers
	CurrentTerm int
	VotedFor    int
	Log         []LogEntry

	// mu is a lock for synchronized access to this struct
	mu sync.Mutex
	// Me is the ID of this specific Node
	Me int
	// Peers is the ID of all other servers in the cluster
	Peers []int
	// Persistence layer
	Persistence Persistence

	// Volatile state on all servers
	CommitIndex int
	LastApplied int

	// State of this current Node
	State RftState

	dead int
}

func (cm *CnsModule) isAlive() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.dead == 1
}

// GetState returns the current term of the specific node and whether it is a  leader
func (cm *CnsModule) GetState() (int, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.CurrentTerm, cm.State == Leader
}

func (cm *CnsModule) electionTimeout() time.Duration {
	// This is longer than the recommended timeout duration in sect 5.2
	// The testing program I am using requires an election to happen in
	// about 5 secs of failure
	return time.Duration(150+rand.Intn(250)) * time.Millisecond
}

func (cm *CnsModule) ticker() {
	for cm.isAlive() == false {
		electionTimeout := cm.electionTimeout()
		startingTerm, startingAsLeader := cm.GetState()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			<-ticker.C

			currentTerm, isLeader := cm.GetState()

			if isLeader {
				// TODO add log here
				return
			}

			// we check that the current Term has not been incremented
			// A follower increments the term once another election has started
			if startingTerm != currentTerm {
				// TODO add another log here

				return
			}

			// Start the election at this point
		}

	}
}
