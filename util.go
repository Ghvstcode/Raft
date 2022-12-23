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
	t         *testing.T
	n         int
	cluster   []*Server
	connected map[int]bool
	start     time.Time // time at which make_config() was called
	finished  int32
	mu        sync.Mutex
	t0        time.Time // time at which test_test.go called cfg.begin()
}

func make_config(t *testing.T, n int) *config {
	runtime.GOMAXPROCS(4)
	cfg := &config{}
	cfg.t = t
	sv := make([]*Server, n)
	cfg.n = n
	ready := make(chan interface{})
	cfg.connected = make(map[int]bool)

	// create a full set of Rafts.
	for i := 0; i < cfg.n; i++ {
		peerIds := make([]int, 0)
		for p := 0; p < n; p++ {
			if p != i {
				peerIds = append(peerIds, p)
			}
		}

		sv[i] = NewServer(i, peerIds, ready)
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
	return cfg
}

func (cfg *config) checkOneLeader() int {
	for iters := 0; iters < 10; iters++ {
		ms := 450 + (rand.Int63() % 100)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		leaders := make(map[int][]int)
		for i := 0; i < cfg.n; i++ {
			if cfg.connected[i] {
				//fmt.Println("connected69")
				//fmt.Println("cfg.cluster[i]", cfg.cluster[i].cm)
				//term, leader := cfg.cluster[i].cm.IsLeader()
				//cm := cfg.cluster[i].cm.
				//lck := cm.mu.TryLock()
				//
				//term := cm.CurrentTerm
				//leader := cm.State == Leader
				//if lck {
				//	fmt.Println("Was locked up")
				//	cm.mu.Unlock()
				//}
				//fmt.Println("Terrm", term)
				//fmt.Println("Leader", leader)
				//if term, leader := cfg.cluster[i].cm.IsLeader(); leader {
				//	leaders[term] = append(leaders[term], i)
				//}
				if term, leader := cfg.cluster[i].cm.IsLeader(); leader {
					//fmt.Println("leader")
					leaders[term] = append(leaders[term], i)
				}
				//fmt.Println("iter", i)
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
