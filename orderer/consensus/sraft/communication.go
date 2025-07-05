package sraft

import (
	"context"
	"github.com/ramendersingh/fabric-sraft/protos/orderer/sraft"
	"google.golang.org/grpc"
)

type Replicator struct {
	conn   *grpc.ClientConn
	client sraft.SRaftClient
}

func (r *Replicator) AppendShard(req *sraft.AppendShardRequest) (*sraft.AppendShardResponse, error) {
	return r.client.AppendShard(context.Background(), req)
}

func NewReplicator(addr string) (*Replicator, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Replicator{
		conn:   conn,
		client: sraft.NewSRaftClient(conn),
	}, nil
}