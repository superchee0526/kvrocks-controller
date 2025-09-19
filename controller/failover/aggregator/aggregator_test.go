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

package aggregator

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/apache/kvrocks-controller/config"
	"github.com/apache/kvrocks-controller/metrics"
	"github.com/apache/kvrocks-controller/store/engine"
	"github.com/apache/kvrocks-controller/store/keys"
)

type testEngine struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newTestEngine() *testEngine {
	return &testEngine{data: make(map[string][]byte)}
}

func (e *testEngine) insert(key string, value []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data[key] = value
}

func (e *testEngine) remove(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.data, key)
}

func (e *testEngine) List(_ context.Context, prefix string) ([]engine.Entry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	trimmed := strings.TrimRight(prefix, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	dedup := make(map[string]struct{})
	var entries []engine.Entry
	for key, value := range e.data {
		if !strings.HasPrefix(key, trimmed) {
			continue
		}
		suffix := strings.TrimPrefix(key, trimmed)
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix == "" {
			continue
		}
		parts := strings.SplitN(suffix, "/", 2)
		child := parts[0]
		if _, ok := dedup[child]; ok {
			continue
		}
		dedup[child] = struct{}{}
		entry := engine.Entry{Key: child}
		if len(parts) == 1 {
			entry.Value = value
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (e *testEngine) Get(context.Context, string) ([]byte, error)  { panic("unused") }
func (e *testEngine) Exists(context.Context, string) (bool, error) { panic("unused") }
func (e *testEngine) Set(context.Context, string, []byte) error    { panic("unused") }
func (e *testEngine) Delete(context.Context, string) error         { panic("unused") }
func (e *testEngine) Close() error                                 { return nil }
func (e *testEngine) ID() string                                   { return "test" }
func (e *testEngine) Leader() string                               { return "test" }
func (e *testEngine) LeaderChange() <-chan bool                    { return make(chan bool) }
func (e *testEngine) IsReady(context.Context) bool                 { return true }

type fakeStore struct {
	namespaces map[string][]string
	engine     engine.Engine
}

func (s *fakeStore) ListNamespace(context.Context) ([]string, error) {
	out := make([]string, 0, len(s.namespaces))
	for ns := range s.namespaces {
		out = append(out, ns)
	}
	return out, nil
}

func (s *fakeStore) ListCluster(_ context.Context, namespace string) ([]string, error) {
	return s.namespaces[namespace], nil
}

func (s *fakeStore) GetEngine() engine.Engine {
	return s.engine
}

func TestAggregatorCachesFreshRecordsOnly(t *testing.T) {
	eng := newTestEngine()
	store := &fakeStore{engine: eng, namespaces: map[string][]string{"ns": {"cluster"}}}

	cfg := &config.ControllerConfig{
		FailOver: &config.FailOverConfig{PingIntervalSeconds: 2, MaxPingCount: 5},
		MultiDetect: &config.MultiDetectConfig{
			Enabled:             true,
			WindowPeriods:       3,
			Quorum:              config.QuorumConfig{Mode: config.QuorumModeMajority},
			KeyPrefix:           "/kvrocks/metadata",
			AggregateIntervalMS: 100,
		},
	}

	agg := New(store, cfg)

	fresh := keys.DetectRecord{Count: 2, TS: time.Now().Unix()}
	freshKey := keys.DetectionKey(cfg.MultiDetect.KeyPrefix, "ns", "cluster", "sessionA", "node-1")
	payload, _ := json.Marshal(fresh)
	eng.insert(freshKey, payload)

	stale := keys.DetectRecord{Count: 4, TS: time.Now().Add(-30 * time.Second).Unix()}
	staleKey := keys.DetectionKey(cfg.MultiDetect.KeyPrefix, "ns", "cluster", "sessionB", "node-1")
	stalePayload, _ := json.Marshal(stale)
	eng.insert(staleKey, stalePayload)

	agg.aggregateOnce(context.Background())

	clusterKey := joinClusterKey("ns", "cluster")
	records := agg.GetFreshRecords(clusterKey, "node-1")
	require.Len(t, records, 1)
	require.Equal(t, fresh.Count, records[0].Count)

	// ensure defensive copy
	records[0].Count = 99
	again := agg.GetFreshRecords(clusterKey, "node-1")
	require.Equal(t, fresh.Count, again[0].Count)

	gaugeVal := testutil.ToFloat64(metrics.MultiDetectQuorumSessions.WithLabelValues("ns", "cluster", "node-1"))
	require.Equal(t, 1.0, gaugeVal)

	// remove data and ensure gauge state cleared on next cycle
	eng.remove(freshKey)
	agg.aggregateOnce(context.Background())
	require.Len(t, agg.gaugeState, 0)
}
