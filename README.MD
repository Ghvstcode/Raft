# RAFT Consensus Protocol in Golang

Welcome to this project, where I attempt to implement the RAFT consensus protocol in Go!

## Why did I do this?

The purpose of this project was purely for my own learning, to gain a deeper understanding of how RAFT works. This implementation is not intended for any sort of production use.

## What is RAFT?

RAFT is a consensus protocol that is used to maintain a replicated log in a distributed system. It was designed to be easy to understand and implement, with a strong focus on clarity.

In a distributed system, it is important to ensure that all nodes have the same data and that the system is able to make progress even in the presence of failures. A replicated log is a data structure that is used to store information in a distributed system, and a consensus protocol is a set of rules that nodes in the system follow to agree on the contents of the log.

The key features of RAFT are:

-   Leader election: RAFT uses an election process to determine which server should be the leader. The leader is responsible for replicating log entries and responding to client requests.
-   Log replication: The leader replicates log entries to the other servers in the cluster, ensuring that they all have the same log. This is done by sending log entries to the other servers and waiting for them to acknowledge receipt.
-   Persistence: RAFT servers persist their log to stable storage, so that they can recover from failures. This means that the log is stored on disk or some other type of stable storage, rather than just in memory.

RAFT is a relatively simple consensus protocol compared to others, such as Paxos. It is designed to be easy to understand and implement, with a strong focus on clarity. It is used in many distributed systems, including distributed databases, message brokers, and distributed file systems.

## How does RAFT work?

In RAFT, there are three main roles that a server can play: leader, follower, or candidate. The leader is responsible for replicating log entries and responding to client requests, while followers simply replicate log entries and vote in elections. Candidates are servers that are in the process of running for the leadership role.

The election process works as follows:

1.  A server becomes a candidate when it starts up or when it has not heard from the leader for a certain amount of time (called the election timeout).
2.  The candidate sends out RequestVote RPCs (remote procedure calls) to the other servers in the cluster, asking for their vote.
3.  If a server receives a RequestVote RPC from a candidate that has a more up-to-date log than it does, it will vote for the candidate.
4.  If a candidate receives votes from a majority of the servers in the cluster, it becomes the leader.

Once a leader has been elected, it begins replicating log entries to the other servers in the cluster. This is done by sending AppendEntries RPCs to the followers, which contain the log entries to be replicated. The followers will acknowledge receipt of the log entries, and the leader will wait for a quorum of followers (a majority) to acknowledge before considering the log entry to be committed.

The persistence feature of RAFT ensures that log entries are stored on stable storage, so that the system can recover from failures. This is important because it means that the system can continue to make progress even if a server crashes or becomes unavailable.

Overall, RAFT is a simple but powerful consensus protocol that is widely used in distributed systems. It provides a way for servers in a cluster to agree on the contents of a replicated log, ensuring that all nodes have the same data and that the system can make progress in the presence of failures.

## My Implementation

My implementation of RAFT was heavily inspired by the following resources:

-   [A blog post by Eli Bendersky](https://eli.thegreenplace.net/2020/implementing-raft-part-0-introduction/)
-   [MIT's 6.824 distributed systems course](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html)

I used the tests from these resources to ensure that my implementation is correct. In addition, my implementation includes the following features:

-   Leader election
-   Log replication
-   Persistence

## Gotchas when implementing RAFT in Golang

Implementing RAFT in Golang can be challenging, and there are a few things to watch out for:

-   Leader election can be tricky to get right, especially if you have multiple servers competing for the leadership role. Make sure to handle edge cases such as when a server becomes partitioned from the rest of the cluster.
-   Log replication requires careful handling of log entries and server states. Pay attention to the order in which you apply log entries and update server states.
-   Persistence is important for fault tolerance, but it can also be a source of bugs if not implemented correctly. Make sure to test your persistence code thoroughly.
-   Be mindful of data races when accessing shared data structures. Make use of mutexes or other synchronization mechanisms as needed.

## Resources

If you want to learn more about RAFT, here are some places you can look:

-   [The original RAFT paper](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf)
-   [The RAFT Wikipedia page](https://en.wikipedia.org/wiki/Raft_(computer_science))
-   [MIT's 6.824 distributed systems course](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html)
-   [A comprehensive list of resources on RAFT](https://raft.github.io/)

## Conclusion
- I hope to fine-tune this implementation in the future!

**part of this readme is written with help from ChatGPT **
