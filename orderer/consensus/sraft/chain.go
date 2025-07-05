package sraft

import (
	"github.com/ramendersingh/fabric-sraft/orderer/consensus"
	"github.com/ramendersingh/fabric-sraft/protos/orderer/sraft"
	"go.uber.org/zap"

)

type Chain struct {
	consensus.ChainSupport
	logger          *zap.Logger
	config          *Config
	zoneID          string
	isRootLeader    bool
	shardReplicator map[string]*ShardReplicator
}

func (c *Chain) Order(env *cb.Envelope, configSeq uint64) error {
	// Submit transaction to sRaft log
	entry := &sraft.LogEntry{
		Data: env.Payload,
		Config: configSeq,
	}
	return c.shards[c.zoneID].Replicate(entry)
}

func (c *Chain) Start() {
	go c.runLeaderElection()
	go c.monitorShards()
}
