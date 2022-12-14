package raft

import (
	"errors"
	"fmt"
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
	// TODO refactor this bit
	lastElectionReset time.Time
	server            Server
}
type AppendEntriesArgs struct {
	// Term is the leaders current term
	Term int
	// LeaderID is the ID of the leader so the follower can redirect clients when a new request comes in
	LeaderID int
	// PrevLogIndex is the index of the log entry immediately preceding new ones
	PrevLogIndex int
	// PrevLogTerm is the term of PrevLogIndex entry
	PrevLogTerm int
	// An array of the log entries to store
	Entries []LogEntry
	// LeaderCommit is the leaders CommitIndex
	LeaderCommit int
}
type Server interface {
	// Call makes an RPC using the provided service method
	Call(id int, service string, args interface{}, res interface{}) error
}

// RVArgs struct represents an argument to be passed to the requestVote RPC call
// It is defined in figure 2 of the paper
type RVArgs struct {
	// candidates Term
	Term int
	// ID of the candidate requesting the vote
	CandidateID int
	// index of the candidates last log entry
	LastLogIndex int
	// term of candidates last log entry
	LastLogTerm int
}

// RVResults represents the response from the RPC call requesting for votes
type RVResults struct {
	// currentTerm for candidate to update itself
	Term int
	// True means candidate received vot
	VoteGranted bool
}

func (cm *CnsModule) isAlive() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.dead == 1
}

// GetState returns the current term of the specific node and whether it is a  leader
func (cm *CnsModule) isLeader() (int, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.CurrentTerm, cm.State == Leader
}

// GetState returns the current term of the specific node and whether it is a  leader
func (cm *CnsModule) GetState() (int, RftState) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.CurrentTerm, cm.State
}

func (cm *CnsModule) electionTimeout() time.Duration {
	// This is longer than the recommended timeout duration in sect 5.2
	// The testing program I am using requires an election to happen in
	// about 5 secs of failure
	return time.Duration(150+rand.Intn(250)) * time.Millisecond
}

// ticker runs in the background of each follow to be able to start an election if it does not..
// receive a heartbeat in time
func (cm *CnsModule) ticker() {
	for cm.isAlive() == false {
		electionTimeout := cm.electionTimeout()
		startingTerm, _ := cm.GetState()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			<-ticker.C

			currentTerm, isLeader := cm.isLeader()

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
			if time.Since(cm.lastElectionReset) >= electionTimeout {
				cm.runElection()
				return
			}
		}

	}
}

func (cm *CnsModule) runElection() {
	// setup all the things that needs to be done for this user to become a Candidate
	// section 5.2

	cm.CurrentTerm += 1
	cm.VotedFor = cm.Me
	cm.lastElectionReset = time.Now()
	cm.State = Candidate

	termAtStart := cm.CurrentTerm

	votes := 1

	for _, peer := range cm.Peers {
		go cm.requestVote(peer, termAtStart, votes)
		votes++
	}
	go cm.ticker()
}

func (cm *CnsModule) requestVote(peerID, term int, votes int) {
	var res RVResults
	q := RVArgs{
		Term:        term,
		CandidateID: cm.Me,
	}
	// TODO add log here
	if err := cm.server.Call(peerID, "", q, &res); err != nil {
		return
	}

	_, state := cm.GetState()
	if state != Candidate {
		return
	}

	// Check to see if the term has changed, if it has, this peer becomes a follower
	if res.Term > term {
		//  update the state of the peer to follower
		cm.setState(Follower, res.Term, -1)

		return
	}

	if res.Term == term {
		if res.VoteGranted {
			if votes*2 > len(cm.Peers)+1 {
				// start leader
				return
			}
		}
	}
}

func (cm *CnsModule) setState(state RftState, term, votedFor int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.State = state
	cm.CurrentTerm = term
	cm.VotedFor = votedFor
	cm.lastElectionReset = time.Now()
}

// RpcCallOrFollower is a method that makes an RPC call to the provided method and becomes a folower if the
// Term gotten from the response(CurrentTerm) is different from the starting Term before the RPC call was made
func (cm *CnsModule) RpcCallOrFollower(state RftState, id, term int, service string, args interface{}, res interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if err := cm.server.Call(id, service, args, res); err == nil {
		if cm.State != state {
			return errors.New(fmt.Sprintf("expected state %s but got state %s", state, cm.State))
		}
		v, ok := res.(RVResults)
		if ok {
			if v.Term > term {
				cm.setState(Follower, term, -1)
			}
		}

		v2, ok := res.(AppendEntriesArgs)
		if ok {
			if v2.Term > term {
				cm.setState(Follower, term, -1)
			}
		}
	}
	return nil
}

// RVArgs struct represents an argument to be passed to the requestVote RPC
