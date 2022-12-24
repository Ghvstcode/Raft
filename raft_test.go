package raft

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const RaftElectionTimeout = 1000 * time.Millisecond

func TestInitialElection(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	cfg.begin("Test 1: initial election")
	// is a leader elected?
	cfg.checkOneLeader()
	// sleep a bit to avoid racing with followers learning of the
	// election, then check that all peers agree on the term.
	time.Sleep(50 * time.Millisecond)
	term1 := cfg.checkTerms()
	if term1 < 1 {
		t.Fatalf("term is %v, but should be at least 1", term1)
	}

	// does the leader+term stay the same if there is no network failure?
	time.Sleep(2 * RaftElectionTimeout)
	term2 := cfg.checkTerms()
	if term1 != term2 {
		fmt.Printf("warning: term changed even though there were no failures")
	}

	// there should still be a leader.
	cfg.checkOneLeader()

	cfg.end()
}

func TestReElection(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	cfg.begin("Test (2A): election after network failure")

	leader1 := cfg.checkOneLeader()

	// if the leader disconnects, a new one should be elected.
	cfg.DisconnectPeer(leader1)

	cfg.checkOneLeader()

	// if the old leader rejoins, that shouldn't
	// disturb the new leader. and the old leader
	// should switch to follower.
	cfg.ReconnectPeer(leader1)
	leader2 := cfg.checkOneLeader()

	// if there's no quorum, no new leader should
	// be elected.
	cfg.DisconnectPeer(leader2)
	cfg.DisconnectPeer((leader2 + 1) % servers)
	time.Sleep(2 * RaftElectionTimeout)

	// check that the one connected server
	// does not think it is the leader.
	cfg.CheckNoLeader()

	// if a quorum arises, it should elect a leader.
	cfg.ReconnectPeer((leader2 + 1) % servers)
	cfg.checkOneLeader()

	// re-join of last node shouldn't prevent leader from existing.
	cfg.ReconnectPeer(leader2)
	cfg.checkOneLeader()

	cfg.end()
}

func TestManyElections2A(t *testing.T) {
	servers := 7
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	cfg.begin("Test (2A): multiple elections")

	cfg.checkOneLeader()

	iters := 10
	for ii := 1; ii < iters; ii++ {
		// disconnect three nodes
		i1 := rand.Int() % servers
		i2 := rand.Int() % servers
		i3 := rand.Int() % servers
		cfg.DisconnectPeer(i1)
		cfg.DisconnectPeer(i2)
		cfg.DisconnectPeer(i3)

		// either the current leader should still be alive,
		// or the remaining four should elect a new one.
		cfg.checkOneLeader()

		cfg.ReconnectPeer(i1)
		cfg.ReconnectPeer(i2)
		cfg.ReconnectPeer(i3)
	}

	cfg.checkOneLeader()

	cfg.end()
}
