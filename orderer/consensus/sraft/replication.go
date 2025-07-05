package sraft

func (c *Chain) replicateEntry(entry *sraft.LogEntry) error {
	shardReq := &sraft.AppendShardRequest{
		Term:        c.currentTerm,
		LeaderId:    c.config.NodeID,
		Entries:     []*sraft.LogEntry{entry},
		LeaderCommit: c.commitIndex,
		ZoneId:      c.zoneID,
	}

	var wg sync.WaitGroup
	for _, node := range c.config.Zones[c.zoneID].Nodes {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			replicator, _ := NewReplicator(addr)
			replicator.AppendShard(shardReq)
		}(node)
	}
	wg.Wait()
	return nil
}