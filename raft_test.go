package raft

import (
	"testing"
)

func TestInitialElection(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers, false, false)
	
}
