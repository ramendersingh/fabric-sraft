package sraft

import (
	"github.com/ramendersingh/fabric-sraft/orderer/consensus"
)

type Chain struct {
	consensus.ChainSupport
	config          *Config
	zoneID          string
	isRootLeader    bool
	shardReplicator *ShardReplicator
}

func (c *Chain) Order(env *cb.Envelope, configSeq uint64) error {
	// Submit transaction to sRaft log
	return c.shardReplicator.Replicate(env)
}

func (c *Chain) Start() {
	go c.runLeaderElection()
	go c.monitorShards()
}
