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
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/apache/kvrocks-controller/config"
	"github.com/apache/kvrocks-controller/controller/failover/scanhelper"
	"github.com/apache/kvrocks-controller/logger"
	"github.com/apache/kvrocks-controller/metrics"
	"github.com/apache/kvrocks-controller/store/engine"
	"github.com/apache/kvrocks-controller/store/keys"
)

type cacheSnapshot struct {
	data map[string]map[string][]keys.DetectRecord
}

type gaugeKey struct {
	Namespace string
	Cluster   string
	Node      string
}

type Aggregator struct {
	store    clusterView
	failCfg  *config.FailOverConfig
	multiCfg *config.MultiDetectConfig

	batchSize int

	cache atomic.Value

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool

	gaugeMu    sync.Mutex
	gaugeState map[gaugeKey]struct{}
}

type clusterView interface {
	ListNamespace(ctx context.Context) ([]string, error)
	ListCluster(ctx context.Context, namespace string) ([]string, error)
	GetEngine() engine.Engine
}

func New(stor clusterView, cfg *config.ControllerConfig) *Aggregator {
	a := &Aggregator{
		store:      stor,
		failCfg:    cfg.FailOver,
		multiCfg:   cfg.MultiDetect,
		batchSize:  256,
		gaugeState: make(map[gaugeKey]struct{}),
	}
	a.cache.Store(cacheSnapshot{data: make(map[string]map[string][]keys.DetectRecord)})
	return a
}

func (a *Aggregator) Start(ctx context.Context) {
	if !a.multiCfg.Enabled {
		return
	}
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.running = true
	a.mu.Unlock()

	a.wg.Add(1)
	go a.loop(runCtx)
}

func (a *Aggregator) Stop() {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	cancel := a.cancel
	a.running = false
	a.mu.Unlock()

	cancel()
	a.wg.Wait()
	a.resetGauges()
}

func (a *Aggregator) loop(ctx context.Context) {
	defer a.wg.Done()
	interval := time.Duration(a.multiCfg.AggregateIntervalMS) * time.Millisecond
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	a.aggregateOnce(ctx)
	for {
		select {
		case <-ticker.C:
			a.aggregateOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (a *Aggregator) aggregateOnce(ctx context.Context) {
	start := time.Now()
	namespaces, err := a.store.ListNamespace(ctx)
	if err != nil {
		logger.Get().With(zap.Error(err)).Warn("Aggregator failed to list namespaces")
		return
	}

	windowSeconds := a.multiCfg.WindowPeriods * a.failCfg.PingIntervalSeconds
	if windowSeconds <= 0 {
		windowSeconds = a.failCfg.PingIntervalSeconds
		if windowSeconds <= 0 {
			windowSeconds = 1
		}
	}
	window := time.Duration(windowSeconds) * time.Second
	now := time.Now()

	snapshot := cacheSnapshot{data: make(map[string]map[string][]keys.DetectRecord)}
	seenGauges := make(map[gaugeKey]struct{})
	totalRecords := 0

	eng := a.store.GetEngine()

	for _, ns := range namespaces {
		clusters, err := a.store.ListCluster(ctx, ns)
		if err != nil {
			logger.Get().With(zap.Error(err), zap.String("namespace", ns)).Warn("Aggregator failed to list clusters")
			continue
		}
		for _, cluster := range clusters {
			prefix := keys.DetectionClusterPrefix(a.multiCfg.KeyPrefix, ns, cluster)
			kvs, err := scanhelper.ScanAll(ctx, eng, prefix, a.batchSize)
			if err != nil {
				logger.Get().With(
					zap.Error(err),
					zap.String("namespace", ns),
					zap.String("cluster", cluster),
				).Warn("Aggregator failed to scan detection prefix")
				continue
			}
			clusterKey := joinClusterKey(ns, cluster)
			for fullKey, payload := range kvs {
				nodeID := path.Base(fullKey)
				if nodeID == "" {
					continue
				}
				var record keys.DetectRecord
				if err := json.Unmarshal(payload, &record); err != nil {
					continue
				}
				if now.Sub(time.Unix(record.TS, 0)) > window {
					continue
				}
				if _, ok := snapshot.data[clusterKey]; !ok {
					snapshot.data[clusterKey] = make(map[string][]keys.DetectRecord)
				}
				snapshot.data[clusterKey][nodeID] = append(snapshot.data[clusterKey][nodeID], record)
				totalRecords++
				gk := gaugeKey{Namespace: ns, Cluster: cluster, Node: nodeID}
				seenGauges[gk] = struct{}{}
			}
		}
	}

	metrics.MultiDetectAggregatorDuration.WithLabelValues("cycle").Observe(float64(time.Since(start).Milliseconds()))
	metrics.MultiDetectAggregatorRecords.Add(float64(totalRecords))

	a.cache.Store(snapshot)
	a.refreshGaugeMetrics(seenGauges, snapshot)
}

func (a *Aggregator) refreshGaugeMetrics(seen map[gaugeKey]struct{}, snapshot cacheSnapshot) {
	a.gaugeMu.Lock()
	defer a.gaugeMu.Unlock()

	for key := range a.gaugeState {
		if _, ok := seen[key]; !ok {
			metrics.MultiDetectQuorumSessions.DeleteLabelValues(key.Namespace, key.Cluster, key.Node)
		}
	}

	for key := range seen {
		clusterKey := joinClusterKey(key.Namespace, key.Cluster)
		count := 0
		if nodes, ok := snapshot.data[clusterKey]; ok {
			count = len(nodes[key.Node])
		}
		metrics.MultiDetectQuorumSessions.WithLabelValues(key.Namespace, key.Cluster, key.Node).Set(float64(count))
	}

	a.gaugeState = seen
}

func (a *Aggregator) resetGauges() {
	a.gaugeMu.Lock()
	defer a.gaugeMu.Unlock()
	for key := range a.gaugeState {
		metrics.MultiDetectQuorumSessions.DeleteLabelValues(key.Namespace, key.Cluster, key.Node)
	}
	a.gaugeState = make(map[gaugeKey]struct{})
}

func (a *Aggregator) GetFreshRecords(clusterKey, nodeID string) []keys.DetectRecord {
	snap := a.cache.Load().(cacheSnapshot)
	nodes := snap.data[clusterKey]
	if len(nodes) == 0 {
		return nil
	}
	records := nodes[nodeID]
	if len(records) == 0 {
		return nil
	}
	out := make([]keys.DetectRecord, len(records))
	copy(out, records)
	return out
}

func joinClusterKey(namespace, cluster string) string {
	return strings.TrimSuffix(namespace, "/") + "/" + cluster
}
