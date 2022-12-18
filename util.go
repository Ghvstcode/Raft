package raft

import (
	"fmt"
	"runtime"
	"testing"
)

type config struct {
	t         *testing.T
	n         int
	cluster   []Server
	connected map[int]bool
}

func make_config(t *testing.T, n int, unreliable bool, snapshot bool) *config {
	runtime.GOMAXPROCS(4)
	cfg := &config{}
	cfg.t = t
	cfg.cluster = make([]Server, n)
	cfg.n = n
	ready := make(chan interface{})

	// create a full set of Rafts.
	for i := 0; i < cfg.n; i++ {
		peerIds := make([]int, 0)
		for p := 0; p < n; p++ {
			if p != i {
				peerIds = append(peerIds, p)
			}
		}

		cfg.cluster[i] = NewServer(i, peerIds, ready)
		cfg.cluster[i].Serve()

	}

	// Connect all peers to each other.
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				cfg.cluster[i].ConnectToPeer(j, cfg.cluster[j].GetListenAddr())
			}
		}
		cfg.connected[i] = true
	}
	close(ready)

	return cfg
}

func startTest(description string) {
	fmt.Printf("%s ...\n", description)
}
