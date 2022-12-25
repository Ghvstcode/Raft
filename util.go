package raft

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type config struct {
	t           *testing.T
	n           int
	cluster     []*Server
	connected   map[int]bool
	start       time.Time // time at which make_config() was called
	finished    int32
	mu          sync.Mutex
	t0          time.Time // time at which test_test.go called cfg.begin()
	applyErr    []string
	logs        []map[int]interface{}
	commitChans []chan CommitEntry
	commits     [][]CommitEntry
}

func make_config(t *testing.T, n int) *config {
	runtime.GOMAXPROCS(4)
	cfg := &config{}
	cfg.t = t
	sv := make([]*Server, n)
	cfg.n = n
	ready := make(chan interface{})
	cfg.connected = make(map[int]bool)
	cfg.logs = make([]map[int]interface{}, cfg.n)

	commitChans := make([]chan CommitEntry, n)
	commits := make([][]CommitEntry, n)

	// create a full set of Rafts.
	for i := 0; i < cfg.n; i++ {
		peerIds := make([]int, 0)
		for p := 0; p < n; p++ {
			if p != i {
				peerIds = append(peerIds, p)
			}
		}

		commitChans[i] = make(chan CommitEntry)
		sv[i] = NewServer(i, peerIds, ready, commitChans[i])
		sv[i].Serve()

	}

	// Connect all peers to each other.
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				sv[i].ConnectToPeer(j, sv[j].GetListenAddr())
			}
		}
		cfg.connected[i] = true

	}
	close(ready)
	cfg.cluster = sv
	cfg.commitChans = commitChans
	cfg.commits = commits

	for i := 0; i < n; i++ {
		go cfg.collectCommits(i)
	}

	return cfg
}

func (cfg *config) collectCommits(i int) {
	for c := range cfg.commitChans[i] {
		cfg.mu.Lock()
		//tlog("collectCommits(%d) got %+v", i, c)
		//tlog("collectCommits(%d) got %+v", i, c)
		cfg.commits[i] = append(cfg.commits[i], c)
		cfg.mu.Unlock()
	}
}

func (cfg *config) checkOneLeader() int {
	for iters := 0; iters < 10; iters++ {
		ms := 450 + (rand.Int63() % 100)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		leaders := make(map[int][]int)
		for i := 0; i < cfg.n; i++ {
			if cfg.connected[i] {
				term, leader := cfg.cluster[i].cm.IsLeader()
				if leader {
					leaders[term] = append(leaders[term], i)
				}
			}
		}

		lastTermWithLeader := -1
		for term, leaders := range leaders {
			if len(leaders) > 1 {
				cfg.t.Fatalf("term %d has %d (>1) leaders", term, len(leaders))
			}
			if term > lastTermWithLeader {
				lastTermWithLeader = term
			}
		}

		if len(leaders) != 0 {
			return leaders[lastTermWithLeader][0]
		}
	}

	fmt.Println("test ln 103")
	cfg.t.Fatalf("expected one leader, got none")
	return -1
}

// check that everyone agrees on the term.
func (cfg *config) checkTerms() int {
	term := -1
	for i := 0; i < cfg.n; i++ {
		if cfg.connected[i] {
			xterm, _ := cfg.cluster[i].cm.GetState()
			if term == -1 {
				term = xterm
			} else if term != xterm {
				cfg.t.Fatalf("servers disagree on term")
			}
		}
	}
	return term
}

func (cfg *config) checkTimeout() {
	// enforce a two minute real-time limit on each test
	if !cfg.t.Failed() && time.Since(cfg.start) > 120*time.Second {
		cfg.t.Fatal("test took longer than 120 seconds")
	}
}

func (cfg *config) end() {
	//cfg.checkTimeout()
	if cfg.t.Failed() == false {
		cfg.mu.Lock()
		t := time.Since(cfg.t0).Seconds() // real time
		npeers := cfg.n                   // number of Raft peers
		cfg.mu.Unlock()

		fmt.Printf("  ... Passed --")
		fmt.Printf("  %4.1f  %d\n", t, npeers)
	}
}

func (cfg *config) cleanup() {
	atomic.StoreInt32(&cfg.finished, 1)
	//cfg.checkTimeout()
}

func (cfg *config) begin(description string) {
	fmt.Printf("%s ...\n", description)
	cfg.t0 = time.Now()
}

func (cfg *config) DisconnectPeer(id int) {
	cfg.cluster[id].DisconnectAllPeers()
	for j := 0; j < cfg.n; j++ {
		if j != id {
			cfg.cluster[j].DisconnectPeer(id)
		}
	}
	cfg.connected[id] = false
}

func (cfg *config) CheckNoLeader() {
	for i := 0; i < cfg.n; i++ {
		if cfg.connected[i] {
			_, isLeader := cfg.cluster[i].cm.IsLeader()
			if isLeader {
				cfg.t.Fatalf("server %d leader; want none", i)
			}
		}
	}
}

func (cfg *config) ReconnectPeer(id int) {
	for j := 0; j < cfg.n; j++ {
		if j != id {
			if err := cfg.cluster[id].ConnectToPeer(j, cfg.cluster[j].GetListenAddr()); err != nil {
				cfg.t.Fatal(err)
			}
			if err := cfg.cluster[j].ConnectToPeer(id, cfg.cluster[id].GetListenAddr()); err != nil {
				cfg.t.Fatal(err)
			}
		}
	}
	cfg.connected[id] = true
}

// how many servers think a log entry is committed?
func (cfg *config) nCommitted(index int) (int, interface{}) {
	count := 0
	var cmd interface{} = nil
	for i := 0; i < len(cfg.cluster); i++ {
		if cfg.applyErr[i] != "" {
			cfg.t.Fatal(cfg.applyErr[i])
		}

		cfg.mu.Lock()
		cmd1, ok := cfg.logs[i][index]
		cfg.mu.Unlock()

		if ok {
			if count > 0 && cmd != cmd1 {
				cfg.t.Fatalf("committed values do not match: index %v, %v, %v",
					index, cmd, cmd1)
			}
			count += 1
			cmd = cmd1
		}
	}
	return count, cmd
}

// do a complete agreement.
// it might choose the wrong leader initially,
// and have to re-submit after giving up.
// entirely gives up after about 10 seconds.
// indirectly checks that the servers agree on the
// same value, since nCommitted() checks this,
// as do the threads that read from applyCh.
// returns index.
// if retry==true, may submit the command multiple
// times, in case a leader fails just after Start().
// if retry==false, calls Start() only once, in order
// to simplify the early Lab 2B tests.
//func (cfg *config) one(cmd interface{}, expectedServers int, retry bool) int {
//	t0 := time.Now()
//	starts := 0
//	for time.Since(t0).Seconds() < 10 && cfg.checkFinished() == false {
//		// try all the servers, maybe one is the leader.
//		index := -1
//		for si := 0; si < cfg.n; si++ {
//			starts = (starts + 1) % cfg.n
//			var rf *Server
//			cfg.mu.Lock()
//			if cfg.connected[starts] {
//				rf = cfg.cluster[starts]
//			}
//			cfg.mu.Unlock()
//			if rf != nil {
//				index1, _, ok := rf.cm.
//				if ok {
//					index = index1
//					break
//				}
//			}
//		}
//
//		if index != -1 {
//			// somebody claimed to be the leader and to have
//			// submitted our command; wait a while for agreement.
//			t1 := time.Now()
//			for time.Since(t1).Seconds() < 2 {
//				nd, cmd1 := cfg.nCommitted(index)
//				if nd > 0 && nd >= expectedServers {
//					// committed
//					if cmd1 == cmd {
//						// and it was the command we submitted.
//						return index
//					}
//				}
//				time.Sleep(20 * time.Millisecond)
//			}
//			if retry == false {
//				cfg.t.Fatalf("one(%v) failed to reach agreement", cmd)
//			}
//		} else {
//			time.Sleep(50 * time.Millisecond)
//		}
//	}
//	if cfg.checkFinished() == false {
//		cfg.t.Fatalf("one(%v) failed to reach agreement", cmd)
//	}
//	return -1
//}

func (cfg *config) checkFinished() bool {
	z := atomic.LoadInt32(&cfg.finished)
	return z != 0
}

func (cfg *config) SubmitToServer(id int, cmd interface{}) bool {
	fmt.Println("submit to server")
	return cfg.cluster[id].cm.Submit(cmd)
}

func (cfg *config) CheckCommittedN(cmd int, n int) {
	nc, _ := cfg.CheckCommitted(cmd)
	if nc != n {
		cfg.t.Errorf("CheckCommittedN got nc=%d, want %d", nc, n)
	}
}

func (cfg *config) CheckCommitted(cmd int) (interface{}, interface{}) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	// Find the length of the commits slice for connected servers.
	commitsLen := -1
	for i := 0; i < cfg.n; i++ {
		if cfg.connected[i] {

			if commitsLen >= 0 {
				// If this was set already, expect the new length to be the same.
				if len(cfg.commits[i]) != commitsLen {
					cfg.t.Fatalf("commits[%d] = %d, commitsLen = %d", i, cfg.commits[i], commitsLen)
				}
			} else {
				commitsLen = len(cfg.commits[i])
			}
		}

		fmt.Println("connected-test-lvl", commitsLen)
	}

	// Check consistency of commits from the start and to the command we're asked
	// about. This loop will return once a command=cmd is found.
	for c := 0; c < commitsLen; c++ {
		cmdAtC := -1
		for i := 0; i < cfg.n; i++ {
			if cfg.connected[i] {
				cmdOfN := cfg.commits[i][c].Command.(int)
				if cmdAtC >= 0 {
					if cmdOfN != cmdAtC {
						cfg.t.Errorf("got %d, want %d at h.commits[%d][%d]", cmdOfN, cmdAtC, i, c)
					}
				} else {
					cmdAtC = cmdOfN
				}
			}
		}
		if cmdAtC == cmd {
			// Check consistency of Index.
			index := -1
			nc := 0
			for i := 0; i < cfg.n; i++ {
				if cfg.connected[i] {
					if index >= 0 && cfg.commits[i][c].Idx != index {
						cfg.t.Errorf("got Index=%d, want %d at h.commits[%d][%d]", cfg.commits[i][c].Idx, index, i, c)
					} else {
						index = cfg.commits[i][c].Idx
					}
					nc++
				}
			}
			return nc, index
		}
	}

	// If there's no early return, we haven't found the command we were looking
	// for.
	cfg.t.Errorf("cmd=%d not found in commits", cmd)
	return -1, -1
}
