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
	"sort"

	"github.com/apache/kvrocks-controller/config"
	"github.com/apache/kvrocks-controller/consts"
	"github.com/apache/kvrocks-controller/controller/failover/decider"
	"github.com/apache/kvrocks-controller/metrics"
	"github.com/apache/kvrocks-controller/store"
)

type candidate struct {
	node store.Node
	seq  uint64
}

func ChoosePromotionTargetWithQuorum(
	ctx context.Context,
	reader decider.RecordReader,
	shard *store.Shard,
	namespace, cluster string,
	cfg *config.ControllerConfig,
) (string, error) {
	if shard == nil {
		return "", consts.ErrShardNoReplica
	}

	masterIdx := -1
	for idx, node := range shard.Nodes {
		if node.IsMaster() {
			masterIdx = idx
			break
		}
	}
	if masterIdx == -1 {
		return "", consts.ErrOldMasterNodeNotFound
	}
	if len(shard.Nodes) <= 1 {
		return "", consts.ErrShardNoReplica
	}

	requireMajority := cfg.MultiDetect.RequireQuorumOnPromotion
	maxFail := cfg.FailOver.MaxPingCount
	clusterKey := namespace + "/" + cluster

	candidates := make([]candidate, 0, len(shard.Nodes)-1)
	for idx, node := range shard.Nodes {
		if idx == masterIdx {
			continue
		}
		info, err := node.GetClusterNodeInfo(ctx)
		if err != nil {
			continue
		}
		if requireMajority && !decider.HasMajorityPass(reader, clusterKey, node.ID(), maxFail) {
			continue
		}
		candidates = append(candidates, candidate{node: node, seq: info.Sequence})
	}

	if len(candidates) == 0 {
		metrics.MultiDetectPromotionBlocked.WithLabelValues("no_quorum_candidate").Inc()
		return "", consts.ErrShardNoMatchNewMaster
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].seq > candidates[j].seq
	})

	return candidates[0].node.ID(), nil
}
