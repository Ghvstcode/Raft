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

	lli, llt := cm.getIndexState()

	if args.Term > term {
		DPrintf("term out of date in RV")
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
	cm.persistToStorage()
	DPrintf("RV reply: %+v", res)
	cm.mu.Unlock()
	return nil
}

func (cm *CnsModule) AppendEntries(args AppendEntriesArgs, res *AppendEntriesReply) error {
	term, state := cm.GetState()
	if state == Dead {
		return nil
	}
	if args.Term > term {
		DPrintf("term out of date in AE")
		cm.setState(Follower, args.Term, -1)
	}

	res.Success = false

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if args.Term == term {
		if state != Follower {
			cm.setState(Follower, args.Term, -1)
		}

		cm.lastElectionReset = time.Now()
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
			}

			if newEntriesIndex < len(args.Entries) {
				cm.Log = append(cm.Log[:logInsertIndex], args.Entries[newEntriesIndex:]...)
				DPrintf("current log state: %v", cm.Log)
			}

			if args.LeaderCommit > cm.CommitIndex {
				min := func(a, b int) int {
					if a < b {
						return a
					}
					return b
				}
				DPrintf("...set commitIndex=%d", cm.CommitIndex)
				cm.CommitIndex = min(args.LeaderCommit, len(cm.Log)-1)
				cm.newCommitReadyChan <- struct{}{}
			}
		}
	}

	res.Term = term
	cm.persistToStorage()
	DPrintf("AE reply: %+v", res)
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
