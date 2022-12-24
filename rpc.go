package raft

import (
	"time"
)

// Proxy is a simple wrapper around the RPC methods
type Proxy struct {
	cm *CnsModule
}

func (pp *Proxy) RequestVote(args RVArgs, res *RVResults) error {
	return pp.cm.RequestVote(args, res)
}

func (pp *Proxy) AppendEntries(args AppendEntriesArgs, res *AppendEntriesReply) error {
	return pp.cm.AppendEntries(args, res)
}

func (cm *CnsModule) RequestVote(args RVArgs, res *RVResults) error {
	term, state := cm.GetState()
	if state == Dead {
		return nil
	}

	if args.Term > term {
		cm.setState(Follower, args.Term, -1)
	}

	res.VoteGranted = false
	cm.mu.Lock()
	if cm.CurrentTerm == args.Term &&
		(cm.VotedFor == -1 || cm.VotedFor == args.CandidateID) {

		res.VoteGranted = true
		cm.VotedFor = args.CandidateID
		cm.lastElectionReset = time.Now()
	}

	res.Term = cm.CurrentTerm
	cm.mu.Unlock()
	return nil
}

func (cm *CnsModule) AppendEntries(args AppendEntriesArgs, res *AppendEntriesReply) error {
	term, state := cm.GetState()
	if state == Dead {
		return nil
	}
	if args.Term > term {
		cm.setState(Follower, args.Term, -1)
	}

	res.Success = false
	if args.Term == term {
		if state != Follower {
			// Become follower
			cm.setState(Follower, args.Term, -1)
		}

		cm.mu.Lock()
		cm.lastElectionReset = time.Now()
		cm.mu.Unlock()
		res.Success = true
	}

	res.Term = term
	return nil
}
