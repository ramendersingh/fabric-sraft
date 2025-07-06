package sraft

func (c *Chain) runLeaderElection() {
	for {
		select {
		case <-c.electionTimer.C:
			if !c.isLeader() {
				c.startElection()
			}
		}
	}
}

func (c *Chain) startElection() {
	c.currentTerm++
	votes := 1 // Vote for self

	for _, node := range c.config.Zones[c.zoneID].Nodes {
		resp, err := c.requestVote(node)
		if err == nil && resp.VoteGranted {
			votes++
		}
	}

	if votes > len(c.config.Zones[c.zoneID].Nodes)/2 {
		c.becomeLeader()
	}
}