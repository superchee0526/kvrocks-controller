/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package failover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/apache/kvrocks-controller/config"
	"github.com/apache/kvrocks-controller/store"
	"github.com/apache/kvrocks-controller/store/keys"
)

type stubNode struct {
	id     string
	master bool
	seq    uint64
}

func (s *stubNode) ID() string                  { return s.id }
func (s *stubNode) Password() string            { return "" }
func (s *stubNode) Addr() string                { return "" }
func (s *stubNode) IsMaster() bool              { return s.master }
func (s *stubNode) SetRole(string)              {}
func (s *stubNode) SetPassword(string)          {}
func (s *stubNode) Reset(context.Context) error { return nil }
func (s *stubNode) GetClusterNodeInfo(context.Context) (*store.ClusterNodeInfo, error) {
	return &store.ClusterNodeInfo{Sequence: s.seq}, nil
}
func (s *stubNode) GetClusterInfo(context.Context) (*store.ClusterInfo, error) { return nil, nil }
func (s *stubNode) SyncClusterInfo(context.Context, *store.Cluster) error      { return nil }
func (s *stubNode) CheckClusterMode(context.Context) (int64, error)            { return 0, nil }
func (s *stubNode) MigrateSlot(context.Context, store.SlotRange, string) error { return nil }
func (s *stubNode) MarshalJSON() ([]byte, error)                               { return []byte("{}"), nil }
func (s *stubNode) UnmarshalJSON([]byte) error                                 { return nil }
func (s *stubNode) GetClusterNodesString(context.Context) (string, error)      { return "", nil }

type stubReader struct {
	votes map[string][]keys.DetectRecord
}

func (s *stubReader) GetFreshRecords(clusterKey, nodeID string) []keys.DetectRecord {
	key := clusterKey + "|" + nodeID
	records := s.votes[key]
	out := make([]keys.DetectRecord, len(records))
	copy(out, records)
	return out
}

func TestChoosePromotionTargetWithQuorum(t *testing.T) {
	shard := &store.Shard{Nodes: []store.Node{
		&stubNode{id: "master", master: true, seq: 5},
		&stubNode{id: "replica-1", seq: 12},
		&stubNode{id: "replica-2", seq: 8},
	}}

	reader := &stubReader{votes: map[string][]keys.DetectRecord{
		"ns/cluster|replica-1": {{Count: 3}, {Count: 0}}, // tie -> not majority
		"ns/cluster|replica-2": {{Count: 0}, {Count: 1}}, // majority pass
	}}

	cfg := &config.ControllerConfig{
		FailOver: &config.FailOverConfig{MaxPingCount: 2},
		MultiDetect: &config.MultiDetectConfig{
			RequireQuorumOnPromotion: true,
		},
	}

	target, err := ChoosePromotionTargetWithQuorum(context.Background(), reader, shard, "ns", "cluster", cfg)
	require.NoError(t, err)
	require.Equal(t, "replica-2", target)

	cfg.MultiDetect.RequireQuorumOnPromotion = false
	target, err = ChoosePromotionTargetWithQuorum(context.Background(), reader, shard, "ns", "cluster", cfg)
	require.NoError(t, err)
	// Highest sequence replica-1 should win without quorum requirement
	require.Equal(t, "replica-1", target)
}

func TestChoosePromotionTargetNoCandidate(t *testing.T) {
	shard := &store.Shard{Nodes: []store.Node{
		&stubNode{id: "master", master: true, seq: 5},
		&stubNode{id: "replica-1", seq: 7},
	}}
	reader := &stubReader{votes: map[string][]keys.DetectRecord{
		"ns/cluster|replica-1": {{Count: 3}, {Count: 2}},
	}}
	cfg := &config.ControllerConfig{
		FailOver: &config.FailOverConfig{MaxPingCount: 2},
		MultiDetect: &config.MultiDetectConfig{
			RequireQuorumOnPromotion: true,
		},
	}

	_, err := ChoosePromotionTargetWithQuorum(context.Background(), reader, shard, "ns", "cluster", cfg)
	require.Error(t, err)
}
