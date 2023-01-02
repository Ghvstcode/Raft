package raft

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// LogEntry represents a log entry.
// It holds information about the term when the entry was received by the leader
// it contains command for the state machine
type LogEntry struct {
	Command interface{}
	Term    int
}

type CommitEntry struct {
	Command interface{}
	Term    int
	Idx     int
}

// Persistence dummy struct
type Persistence interface {
	Set(key string, value []byte)

	Get(key string) ([]byte, bool)

	IsSet() bool
}

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
	iserver           IServer
	// Volatile state on all leaders
	// NextIndex for each server, is the index of the next log entry to send to that server (initialized to leader
	//last log index + 1)
	NextIndex          map[int]int
	MatchIndex         map[int]int
	newCommitReadyChan chan struct{}
	CommitExecChan     chan<- CommitEntry
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

type AppendEntriesReply struct {
	// currentTerm, for leader to update itself
	Term int
	// true if follower contained entry matching prevLogIndex and prevLogTerm
	Success bool
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

func NewConsensusModule(id int, peerIds []int, server *Server, ready <-chan interface{}, cme chan<- CommitEntry, kv Persistence) *CnsModule {
	cm := new(CnsModule)
	cm.Me = id
	cm.Peers = peerIds
	cm.iserver = server
	cm.State = Follower
	cm.Persistence = kv
	cm.VotedFor = -1
	cm.LastApplied = -1
	cm.CommitIndex = -1
	cm.NextIndex = make(map[int]int)
	cm.MatchIndex = make(map[int]int)
	cm.newCommitReadyChan = make(chan struct{}, 16)
	cm.CommitExecChan = cme

	if cm.Persistence.IsSet() {
		cm.restoreFromStorage()
	}

	go func() {
		<-ready
		cm.mu.Lock()
		cm.lastElectionReset = time.Now()
		cm.mu.Unlock()
		cm.ticker()
	}()
	go cm.commitChanSender()
	return cm
}

func (cm *CnsModule) isAlive() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.dead == 1
}

// GetState returns the current term of the specific node and whether it is a  leader
func (cm *CnsModule) IsLeader() (int, bool) {
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
	return time.Duration(150+rand.Intn(150)) * time.Millisecond
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
			currentTerm, isLeader := cm.IsLeader()
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
			cm.mu.Lock()
			if time.Since(cm.lastElectionReset) >= electionTimeout {
				cm.mu.Unlock()
				cm.runElection()

				return
			}

			cm.mu.Unlock()
		}

	}
}

func (cm *CnsModule) runElection() {
	// setup all the things that needs to be done for this user to become a Candidate
	// section 5.2
	cm.setState(Candidate, cm.CurrentTerm, cm.Me)

	var m sync.Mutex

	termAtStart := cm.CurrentTerm
	votes := 1

	for _, peer := range cm.Peers {
		go cm.requestVote(peer, termAtStart, votes, cm.Me)
		m.Lock()
		votes++
		m.Unlock()

	}
	go cm.ticker()
}

func (cm *CnsModule) requestVote(peerID, term, votes, candidate int) {
	lli, llt := cm.getIndexState()
	var res RVResults
	q := RVArgs{
		Term:         term,
		CandidateID:  candidate,
		LastLogIndex: lli,
		LastLogTerm:  llt,
	}

	// TODO add log here
	if err := cm.RpcCallOrFollower(Candidate, peerID, term, "CnsModule.RequestVote", q, &res); err != nil {
		//TODO LOG ERROR
		//fmt.Println("ERR245", err)
		return
	}

	if res.Term == term {
		if res.VoteGranted {
			nv := votes + 1
			if nv*2 > len(cm.Peers)+1 {
				cm.LeaderOps()
				return
			}

		}
	}
}

func (cm *CnsModule) setState(state RftState, term, votedFor int) {
	cm.mu.Lock()
	cm.State = state
	if state == Candidate {
		cm.CurrentTerm += 1
	} else {
		cm.CurrentTerm = term
	}

	cm.VotedFor = votedFor
	cm.lastElectionReset = time.Now()
	cm.mu.Unlock()

	go cm.ticker()
}

// RpcCallOrFollower is a method that makes an RPC call to the provided method and becomes a folower if the
// Term gotten from the response(CurrentTerm) is different from the starting Term before the RPC call was made
func (cm *CnsModule) RpcCallOrFollower(state RftState, id, term int, service string, args interface{}, res interface{}) error {
	if err := cm.iserver.Call(id, service, args, res); err == nil {
		v, ok := res.(*RVResults)
		if ok {
			_, currentState := cm.GetState()
			if currentState != state {
				return errors.New(fmt.Sprintf("expected state %s but got state %s", state, currentState))
			}

			if v.Term > term {
				cm.setState(Follower, v.Term, -1)
				return errors.New("peer has become follower")
			}
		}

		v2, ok := res.(*AppendEntriesReply)
		if v2 != nil && ok {
			if v2.Term > term {
				cm.setState(Follower, v2.Term, -1)
				return errors.New("peer has become follower")
			}

			cm.appendOps(*v2, term, id, args)
		}
	} else {
		return err
	}
	return nil
}

func (cm *CnsModule) appendOps(res AppendEntriesReply, savedTerm, id int, args interface{}) {
	cm.mu.Lock()
	if cm.State == Leader && savedTerm == res.Term {
		if res.Success {
			vsth, ok := args.(AppendEntriesArgs)
			if ok {

				v0 := vsth
				cm.NextIndex[id] = cm.NextIndex[id] + len(v0.Entries)
				cm.MatchIndex[id] = cm.NextIndex[id] - 1

				saveCmIdx := cm.CommitIndex

				for i := cm.CommitIndex + 1; i < len(cm.Log); i++ {
					if cm.Log[i].Term == cm.CurrentTerm {
						matchCount := 1
						for _, peerId := range cm.Peers {
							if cm.MatchIndex[peerId] >= i {
								matchCount++
							}
						}
						if matchCount*2 > len(cm.Peers)+1 {
							cm.CommitIndex = i
						}

						if cm.CommitIndex != saveCmIdx {
							cm.newCommitReadyChan <- struct{}{}
						}
					} else {
						cm.NextIndex[id] = v0.PrevLogIndex - 2
					}

				}
			}
		}
	}
	cm.mu.Unlock()
}

func (cm *CnsModule) commitChanSender() {
	for range cm.newCommitReadyChan {
		// Find which entries we have to apply.
		cm.mu.Lock()
		savedTerm := cm.CurrentTerm
		savedLastApplied := cm.LastApplied
		var entries []LogEntry
		if cm.CommitIndex > cm.LastApplied {
			entries = cm.Log[cm.LastApplied+1 : cm.CommitIndex+1]
			cm.LastApplied = cm.CommitIndex
		}
		cm.mu.Unlock()

		for i, entry := range entries {
			cm.CommitExecChan <- CommitEntry{
				Command: entry.Command,
				Idx:     savedLastApplied + i + 1,
				Term:    savedTerm,
			}
		}
	}
}

func (cm *CnsModule) sendLeaderHeartbeats() {
	cm.mu.Lock()
	savedTerm := cm.CurrentTerm
	if cm.State != Leader {
		cm.mu.Unlock()
		return
	}
	cm.mu.Unlock()

	for _, peer := range cm.Peers {
		cm.mu.Lock()
		nextIdx := cm.NextIndex[peer]
		prevLogTerm := -1
		prevLogIndex := nextIdx - 1
		if prevLogIndex >= 0 && prevLogIndex <= len(cm.Log)-1 {
			prevLogTerm = cm.Log[prevLogIndex].Term
		}
		entries := cm.Log[nextIdx:]
		q := AppendEntriesArgs{
			Term:         savedTerm,
			LeaderID:     cm.Me,
			PrevLogIndex: prevLogIndex,
			PrevLogTerm:  prevLogTerm,
			Entries:      entries,
			LeaderCommit: cm.CommitIndex,
		}

		cm.mu.Unlock()
		var res AppendEntriesReply
		go cm.RpcCallOrFollower(Follower, peer, savedTerm, "CnsModule.AppendEntries", q, &res)
	}
}

func (cm *CnsModule) LeaderOps() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.State = Leader

	// When a server becomes a leader, it needs to track the next index for any new commands
	// the next index is the location of any new entries to the server
	// For a server with No-logs, it becomes 0
	for _, peerId := range cm.Peers {
		cm.NextIndex[peerId] = len(cm.Log)
		cm.MatchIndex[peerId] = -1
	}

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			cm.sendLeaderHeartbeats()
			<-ticker.C
			_, state := cm.GetState()
			if state != Leader {
				return
			}
		}
	}()
}

func (cm *CnsModule) Submit(command interface{}) bool {
	_, state := cm.GetState()
	if state == Leader {
		cm.mu.Lock()
		cm.persistToStorage()
		cm.Log = append(cm.Log, LogEntry{Command: command, Term: cm.CurrentTerm})
		cm.mu.Unlock()
		return true
	}
	return false
}

func (cm *CnsModule) restoreFromStorage() {
	if termData, found := cm.Persistence.Get("currentTerm"); found {
		d := gob.NewDecoder(bytes.NewBuffer(termData))
		if err := d.Decode(&cm.CurrentTerm); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("currentTerm not found in storage")
	}
	if votedData, found := cm.Persistence.Get("votedFor"); found {
		d := gob.NewDecoder(bytes.NewBuffer(votedData))
		if err := d.Decode(&cm.VotedFor); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("votedFor not found in storage")
	}
	if logData, found := cm.Persistence.Get("log"); found {
		d := gob.NewDecoder(bytes.NewBuffer(logData))
		if err := d.Decode(&cm.Log); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("log not found in storage")
	}
}

func (cm *CnsModule) persistToStorage() {
	var termData bytes.Buffer
	if err := gob.NewEncoder(&termData).Encode(cm.CurrentTerm); err != nil {
		log.Fatal(err)
	}
	cm.Persistence.Set("currentTerm", termData.Bytes())

	var votedData bytes.Buffer
	if err := gob.NewEncoder(&votedData).Encode(cm.VotedFor); err != nil {
		log.Fatal(err)
	}
	cm.Persistence.Set("votedFor", votedData.Bytes())

	var logData bytes.Buffer
	if err := gob.NewEncoder(&logData).Encode(cm.Log); err != nil {
		log.Fatal(err)
	}
	cm.Persistence.Set("log", logData.Bytes())
}
