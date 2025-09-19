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

package decider

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/apache/kvrocks-controller/metrics"
	"github.com/apache/kvrocks-controller/store/keys"
)

type mockAggregator struct {
	data map[string]map[string][]keys.DetectRecord
}

func (m *mockAggregator) GetFreshRecords(clusterKey, nodeID string) []keys.DetectRecord {
	if cluster, ok := m.data[clusterKey]; ok {
		if recs, ok := cluster[nodeID]; ok {
			out := make([]keys.DetectRecord, len(recs))
			copy(out, recs)
			return out
		}
	}
	return nil
}

func TestJudgeNodeStateQuorum(t *testing.T) {
	aggr := &mockAggregator{data: map[string]map[string][]keys.DetectRecord{
		"ns/cluster": {
			"node-1": {
				{Count: 3},
				{Count: 4},
				{Count: 1},
			},
			"node-2": {
				{Count: 0},
				{Count: 1},
				{Count: 0},
			},
			"node-3": {
				{Count: 0},
				{Count: 2},
			},
		},
	}}

	beforeDown := testutil.ToFloat64(metrics.MultiDetectDecisionTotal.WithLabelValues(string(DownByQuorum)))
	beforeUp := testutil.ToFloat64(metrics.MultiDetectDecisionTotal.WithLabelValues(string(UpByQuorum)))
	beforeSelf := testutil.ToFloat64(metrics.MultiDetectDecisionTotal.WithLabelValues(string(UpBySelf)))

	// fail votes > pass votes => down
	decision := JudgeNodeStateQuorum(aggr, "ns/cluster", "node-1", 2)
	require.Equal(t, DownByQuorum, decision)

	// pass votes > fail votes => up by quorum
	decision = JudgeNodeStateQuorum(aggr, "ns/cluster", "node-2", 2)
	require.Equal(t, UpByQuorum, decision)

	// tie => up by self
	decision = JudgeNodeStateQuorum(aggr, "ns/cluster", "node-3", 2)
	require.Equal(t, UpBySelf, decision)

	require.InDelta(t, beforeDown+1, testutil.ToFloat64(metrics.MultiDetectDecisionTotal.WithLabelValues(string(DownByQuorum))), 0.01)
	require.InDelta(t, beforeUp+1, testutil.ToFloat64(metrics.MultiDetectDecisionTotal.WithLabelValues(string(UpByQuorum))), 0.01)
	require.InDelta(t, beforeSelf+1, testutil.ToFloat64(metrics.MultiDetectDecisionTotal.WithLabelValues(string(UpBySelf))), 0.01)
}

func TestHasMajorityPass(t *testing.T) {
	aggr := &mockAggregator{data: map[string]map[string][]keys.DetectRecord{
		"ns/cluster": {
			"node-1": {{Count: 0}, {Count: 1}},
			"node-2": {{Count: 3}, {Count: 4}},
			"node-3": {{Count: 2}, {Count: 2}},
		},
	}}

	require.True(t, HasMajorityPass(aggr, "ns/cluster", "node-1", 2))
	require.False(t, HasMajorityPass(aggr, "ns/cluster", "node-2", 2))
	require.False(t, HasMajorityPass(aggr, "ns/cluster", "node-3", 2))
}
