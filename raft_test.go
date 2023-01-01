package raft

import (
	"fmt"
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

//func TestManyElections(t *testing.T) {
//	servers := 7
//	cfg := make_config(t, servers)
//	defer cfg.cleanup()
//
//	cfg.begin("Test (2A): multiple elections")
//
//	cfg.checkOneLeader()
//
//	iters := 2
//	for ii := 1; ii < iters; ii++ {
//		// disconnect three nodes
//		i1 := rand.Int() % servers
//		i2 := rand.Int() % servers
//		i3 := rand.Int() % servers
//		cfg.DisconnectPeer(i1)
//		cfg.DisconnectPeer(i2)
//		cfg.DisconnectPeer(i3)
//
//		// either the current leader should still be alive,
//		// or the remaining four should elect a new one.
//		cfg.checkOneLeader()
//
//		cfg.ReconnectPeer(i1)
//		cfg.ReconnectPeer(i2)
//		cfg.ReconnectPeer(i3)
//	}
//
//	cfg.checkOneLeader()
//
//	cfg.end()
//}

func TestCommitOneCommand(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	cfg.begin("Test (2B): basic agreement")

	origLeaderId := cfg.checkOneLeader()

	isLeader := cfg.SubmitToServer(origLeaderId, 42)
	if !isLeader {
		t.Errorf("want id=%d leader, but it's not", origLeaderId)
	}

	time.Sleep(time.Duration(150) * time.Millisecond)
	cfg.CheckCommittedN(42, 3)
}

func TestSubmitNonLeaderFails(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	origLeaderId := cfg.checkOneLeader()
	sid := (origLeaderId + 1) % 3
	isLeader := cfg.SubmitToServer(sid, 42)
	if isLeader {
		t.Errorf("want id=%d !leader, but it is", sid)
	}
	time.Sleep(30 * time.Millisecond)
}

func TestCommitMultipleCommands(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	origLeaderId := cfg.checkOneLeader()

	values := []int{42, 55, 81}
	for _, v := range values {
		isLeader := cfg.SubmitToServer(origLeaderId, v)
		if !isLeader {
			t.Errorf("want id=%d leader, but it's not", origLeaderId)
		}
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(150 * time.Millisecond)
	nc, i1 := cfg.CheckCommitted(42)
	_, i2 := cfg.CheckCommitted(55)
	if nc != 3 {
		t.Errorf("want nc=3, got %d", nc)
	}
	if i1 >= i2 {
		t.Errorf("want i1<i2, got i1=%d i2=%d", i1, i2)
	}

	_, i3 := cfg.CheckCommitted(81)
	if i2 >= i3 {
		t.Errorf("want i2<i3, got i2=%d i3=%d", i2, i3)
	}
}

func TestCommitWithDisconnectionAndRecover(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	// Submit a couple of values to a fully connected cluster.
	origLeaderId := cfg.checkOneLeader()
	cfg.SubmitToServer(origLeaderId, 5)
	cfg.SubmitToServer(origLeaderId, 6)

	time.Sleep(250 * time.Millisecond)
	cfg.CheckCommittedN(6, 3)

	dPeerId := (origLeaderId + 1) % 3
	cfg.DisconnectPeer(dPeerId)
	time.Sleep(250 * time.Millisecond)

	// Submit a new command; it will be committed but only to two servers.
	cfg.SubmitToServer(origLeaderId, 7)
	time.Sleep(250 * time.Millisecond)
	cfg.CheckCommittedN(7, 2)

	// Now reconnect dPeerId and wait a bit; it should find the new command too.
	cfg.ReconnectPeer(dPeerId)
	time.Sleep(200 * time.Millisecond)
	cfg.checkOneLeader()

	time.Sleep(150 * time.Millisecond)
	//cfg.CheckCommittedN(7, 3)
}

func TestNoCommitWithNoQuorum(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	// Submit a couple of values to a fully connected cluster.
	origLeaderId := cfg.checkOneLeader()
	cfg.SubmitToServer(origLeaderId, 5)
	cfg.SubmitToServer(origLeaderId, 6)

	time.Sleep(250 * time.Millisecond)
	cfg.CheckCommittedN(6, 3)

	// Disconnect both followers.
	dPeer1 := (origLeaderId + 1) % 3
	dPeer2 := (origLeaderId + 2) % 3
	cfg.DisconnectPeer(dPeer1)
	cfg.DisconnectPeer(dPeer2)
	time.Sleep(250 * time.Millisecond)

	cfg.SubmitToServer(origLeaderId, 8)
	time.Sleep(250 * time.Millisecond)
	cfg.CheckNotCommitted(8)

	// Reconnect both other servers, we'll have quorum now.
	cfg.ReconnectPeer(dPeer1)
	cfg.ReconnectPeer(dPeer2)
	time.Sleep(600 * time.Millisecond)

	// 8 is still not committed because the term has changed.
	cfg.CheckNotCommitted(8)

	// A new leader will be elected. It could be a different leader, even though
	// the original's log is longer, because the two reconnected peers can elect
	// the original's log is longer, because the two reconnected peers can elect
	// each other.
	newLeaderId := cfg.checkOneLeader()

	// But new values will be committed for sure...
	cfg.SubmitToServer(newLeaderId, 9)
	cfg.SubmitToServer(newLeaderId, 10)
	cfg.SubmitToServer(newLeaderId, 11)
	time.Sleep(350 * time.Millisecond)

	for _, v := range []int{9, 10, 11} {
		cfg.CheckCommittedN(v, 3)
	}
}

func TestCrashFollower(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers)
	defer cfg.cleanup()

	origLeaderId := cfg.checkOneLeader()
	cfg.SubmitToServer(origLeaderId, 5)

	time.Sleep(350 * time.Millisecond)
	cfg.CheckCommittedN(5, 3)

	cfg.CrashPeer((origLeaderId + 1) % 3)
	time.Sleep(350 * time.Millisecond)
	cfg.CheckCommittedN(5, 2)
}

