package raft

import (
	"fmt"
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
	//cm.mu.Lock()
	//defer cm.mu.Unlock()
	//term, state := cm.GetState()
	if cm.State == Dead {
		return nil
	}

	if args.Term > cm.CurrentTerm {
		fmt.Println("become follower", args.Term, cm.CurrentTerm)
		// Become follower
		cm.setState(Follower, args.Term, -1)

	}
	res.VoteGranted = false
	//fmt.Println("VOTEDFOR", cm.VotedFor)
	cm.mu.Lock()
	if cm.CurrentTerm == args.Term &&
		(cm.VotedFor == -1 || cm.VotedFor == args.CandidateID) {

		res.VoteGranted = true
		cm.VotedFor = args.CandidateID
		cm.lastElectionReset = time.Now()
		//cm.mu.Unlock()
	}
	cm.mu.Unlock()
	//if args.Term == term {
	//	if state != Follower {
	//		// Become follower
	//		fmt.Println("args term", args.Term, term)
	//		cm.setState(Follower, term, -1)
	//	}
	//}

	res.Term = cm.CurrentTerm
	fmt.Println("RESTERM", res.Term)
	return nil
}

func (cm *CnsModule) AppendEntries(args AppendEntriesArgs, res *AppendEntriesReply) error {
	//cm.mu.Lock()
	//defer cm.mu.Unlock()
	term, state := cm.GetState()
	if state == Dead {
		return nil
	}
	if args.Term > term {
		// Become follower
		cm.setState(Follower, args.Term, -1)
	}
	res.Success = false
	if args.Term == term {
		if state != Follower {
			fmt.Println("I was hitttt76")
			// Become follower
			cm.setState(Follower, args.Term, -1)
		}
		fmt.Println("I was hitttt80AT")
		cm.mu.Lock()
		cm.lastElectionReset = time.Now()
		cm.mu.Unlock()
		res.Success = true
	}
	res.Term = term
	return nil
}
