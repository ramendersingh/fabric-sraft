/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	ab "github.com/hyperledger/fabric-protos-go-apiv2/orderer"
	"github.com/hyperledger/fabric/common/deliver"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/orderer/common/broadcast"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/common/msgprocessor"
	"github.com/hyperledger/fabric/orderer/common/multichannel"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type broadcastSupport struct {
	*multichannel.Registrar
}

func (bs broadcastSupport) BroadcastChannelSupport(msg *cb.Envelope) (*cb.ChannelHeader, bool, broadcast.ChannelSupport, error) {
	return bs.Registrar.BroadcastChannelSupport(msg)
}

type deliverSupport struct {
	*multichannel.Registrar
}

func (ds deliverSupport) GetChain(chainID string) deliver.Chain {
	chain := ds.Registrar.GetChain(chainID)
	if chain == nil {
		return nil
	}
	return chain
}

type server struct {
	bh    *broadcast.Handler
	dh    *deliver.Handler
	debug *localconfig.Debug
	*multichannel.Registrar
}

type responseSender struct {
	ab.AtomicBroadcast_DeliverServer
}

func (rs *responseSender) SendStatusResponse(status cb.Status) error {
	reply := &ab.DeliverResponse{
		Type: &ab.DeliverResponse_Status{Status: status},
	}
	return rs.Send(reply)
}

// SendBlockResponse sends block data and ignores pvtDataMap.
func (rs *responseSender) SendBlockResponse(
	block *cb.Block,
	channelID string,
	chain deliver.Chain,
	signedData *protoutil.SignedData,
) error {
	response := &ab.DeliverResponse{
		Type: &ab.DeliverResponse_Block{Block: block},
	}
	return rs.Send(response)
}

func (rs *responseSender) DataType() string {
	return "block"
}

// ----------------- sraft Changes Date:5-7-2025 Start block ------------------------------

// // NewServer creates an ab.AtomicBroadcastServer based on the broadcast target and ledger Reader
// func NewServer(
// 	r *multichannel.Registrar,
// 	metricsProvider metrics.Provider,
// 	debug *localconfig.Debug,
// 	timeWindow time.Duration,
// 	mutualTLS bool,
// 	expirationCheckDisabled bool,
// ) ab.AtomicBroadcastServer {
// 	s := &server{
// 		dh: deliver.NewHandler(deliverSupport{Registrar: r}, timeWindow, mutualTLS, deliver.NewMetrics(metricsProvider), expirationCheckDisabled),
// 		bh: &broadcast.Handler{
// 			SupportRegistrar: broadcastSupport{Registrar: r},
// 			Metrics:          broadcast.NewMetrics(metricsProvider),
// 		},
// 		debug:     debug,
// 		Registrar: r,
// 	}
// 	return s
// }

// sraft Changes Date:5-7-2025 Start block ------------------------------

func NewServer(conf *config.TopLevel) (*Server, error) {
	// Load sRaft if configured
	consenter, err := consensus.GetPlugin(conf.Consensus)
	if err != nil {
		return nil, err
	}
	return &Server{Consenter: consenter}, nil
	}
	// Default to another consensus (e.g., Raft)
	return raft.New(consensusConfig)
}

// ------------------------- sraft Changes Date:5-7-2025 End block 

func CreateThrottlers(throttleConfig localconfig.Throttling) (RateLimiter, RateLimiter) {
	clientRateLimiter := newRateLimiter(throttleConfig.Rate, throttleConfig.InactivityTimeout)
	orgRateLimiter := newRateLimiter(throttleConfig.Rate, throttleConfig.InactivityTimeout)
	return clientRateLimiter, orgRateLimiter
}

type msgTracer struct {
	function string
	debug    *localconfig.Debug
}

func (mt *msgTracer) trace(traceDir string, msg *cb.Envelope, err error) {
	if err != nil {
		return
	}

	now := time.Now().UnixNano()
	path := fmt.Sprintf("%s%c%d_%p.%s", traceDir, os.PathSeparator, now, msg, mt.function)
	logger.Debugf("Writing %s request trace to %s", mt.function, path)
	go func() {
		if msg == nil {
			logger.Debugf("Error marshaling trace msg for %s: proto: Marshal called with nil", path)
			return
		}
		pb, err := proto.Marshal(msg)
		if err != nil {
			logger.Debugf("Error marshaling trace msg for %s: %s", path, err)
			return
		}
		err = os.WriteFile(path, pb, 0o660)
		if err != nil {
			logger.Debugf("Error writing trace msg for %s: %s", path, err)
		}
	}()
}

type broadcastMsgTracer struct {
	ab.AtomicBroadcast_BroadcastServer
	msgTracer
}

func (bmt *broadcastMsgTracer) Recv() (*cb.Envelope, error) {
	msg, err := bmt.AtomicBroadcast_BroadcastServer.Recv()
	if traceDir := bmt.debug.BroadcastTraceDir; traceDir != "" {
		bmt.trace(bmt.debug.BroadcastTraceDir, msg, err)
	}
	return msg, err
}

type deliverMsgTracer struct {
	deliver.Receiver
	msgTracer
}

func (dmt *deliverMsgTracer) Recv() (*cb.Envelope, error) {
	msg, err := dmt.Receiver.Recv()
	if traceDir := dmt.debug.DeliverTraceDir; traceDir != "" {
		dmt.trace(traceDir, msg, err)
	}
	return msg, err
}

// Broadcast receives a stream of messages from a client for ordering
func (s *server) Broadcast(srv ab.AtomicBroadcast_BroadcastServer) error {
	logger.Debugf("Starting new Broadcast handler")
	defer func() {
		if r := recover(); r != nil {
			logger.Criticalf("Broadcast client triggered panic: %s\n%s", r, debug.Stack())
		}
		logger.Debugf("Closing Broadcast stream")
	}()
	return s.bh.Handle(&broadcastMsgTracer{
		AtomicBroadcast_BroadcastServer: srv,
		msgTracer: msgTracer{
			debug:    s.debug,
			function: "Broadcast",
		},
	})
}

// Deliver sends a stream of blocks to a client after ordering
func (s *server) Deliver(srv ab.AtomicBroadcast_DeliverServer) error {
	logger.Debugf("Starting new Deliver handler")
	defer func() {
		if r := recover(); r != nil {
			logger.Criticalf("Deliver client triggered panic: %s\n%s", r, debug.Stack())
		}
		logger.Debugf("Closing Deliver stream")
	}()

	policyChecker := func(env *cb.Envelope, channelID string) error {
		chain := s.GetChain(channelID)
		if chain == nil {
			return errors.Errorf("channel %s not found", channelID)
		}
		// In maintenance mode, we typically require the signature of /Channel/Orderer/Readers.
		// This will block Deliver requests from peers (which normally satisfy /Channel/Readers).
		sf := msgprocessor.NewSigFilter(policies.ChannelReaders, policies.ChannelOrdererReaders, chain)
		return sf.Apply(env)
	}
	deliverServer := &deliver.Server{
		PolicyChecker: deliver.PolicyCheckerFunc(policyChecker),
		Receiver: &deliverMsgTracer{
			Receiver: srv,
			msgTracer: msgTracer{
				debug:    s.debug,
				function: "Deliver",
			},
		},
		ResponseSender: &responseSender{
			AtomicBroadcast_DeliverServer: srv,
		},
	}
	return s.dh.Handle(srv.Context(), deliverServer)
}
