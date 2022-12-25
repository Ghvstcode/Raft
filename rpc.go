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
	term, state := cm.GetState()
	if state == Dead {
		return nil
	}

	lli, llt := cm.getIndexState()

	if args.Term > term {
		cm.setState(Follower, args.Term, -1)
	}

	res.VoteGranted = false
	cm.mu.Lock()
	if cm.CurrentTerm == args.Term &&
		(cm.VotedFor == -1 || cm.VotedFor == args.CandidateID) &&
		(args.LastLogTerm > llt ||
			(args.LastLogTerm == llt && args.LastLogIndex >= lli)) {

		res.VoteGranted = true
		cm.VotedFor = args.CandidateID
		cm.lastElectionReset = time.Now()
	}

	res.Term = cm.CurrentTerm
	cm.mu.Unlock()
	return nil
}

func (cm *CnsModule) AppendEntries(args AppendEntriesArgs, res *AppendEntriesReply) error {
	//fmt.Println("welcome to this function")
	term, state := cm.GetState()
	if state == Dead {
		return nil
	}
	//fmt.Println("ln-55")
	if args.Term > term {
		//fmt.Println("state was set or sth-pre")
		cm.setState(Follower, args.Term, -1)
		//fmt.Println("state was set or sth")
	}

	res.Success = false
	//fmt.Println("still getting hit at this point 61")

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if args.Term == term {
		if state != Follower {
			// Become follower
			//fmt.Println("become a follower")
			cm.setState(Follower, args.Term, -1)
		}
		//fmt.Println("before LOCKKKK")

		//fmt.Println("AFTER LOCKKKK")
		cm.lastElectionReset = time.Now()
		fmt.Println("pREVLOGIDX")
		if args.PrevLogIndex == -1 ||
			(args.PrevLogIndex < len(cm.Log) && args.PrevLogTerm == cm.Log[args.PrevLogIndex].Term) {

			res.Success = true

			logInsertIndex := args.PrevLogIndex + 1
			newEntriesIndex := 0

			for {
				if logInsertIndex >= len(cm.Log) || newEntriesIndex >= len(args.Entries) {
					break
				}
				if cm.Log[logInsertIndex].Term != args.Entries[newEntriesIndex].Term {
					break
				}
				logInsertIndex++
				newEntriesIndex++
				fmt.Println("appending95", newEntriesIndex)
			}

			if newEntriesIndex < len(args.Entries) {
				cm.Log = append(cm.Log[:logInsertIndex], args.Entries[newEntriesIndex:]...)
			}
			fmt.Println("args.LeaderCommit > cm.CommitIndex", args.LeaderCommit, cm.CommitIndex, args.LeaderCommit > cm.CommitIndex)
			if args.LeaderCommit > cm.CommitIndex {
				min := func(a, b int) int {
					if a < b {
						return a
					}
					return b
				}
				cm.CommitIndex = min(args.LeaderCommit, len(cm.Log)-1)
				fmt.Println("110-here")
				cm.newCommitReadyChan <- struct{}{}
			}
		}

		//cm.mu.Lock()
	}

	res.Term = term
	return nil
}

func (cm *CnsModule) getIndexState() (int, int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if len(cm.Log) > 0 {
		lastIndex := len(cm.Log) - 1
		return lastIndex, cm.Log[lastIndex].Term
	} else {
		return -1, -1
	}
}
