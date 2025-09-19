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

package detect

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/apache/kvrocks-controller/config"
	"github.com/apache/kvrocks-controller/logger"
	"github.com/apache/kvrocks-controller/metrics"
	"github.com/apache/kvrocks-controller/store"
	"github.com/apache/kvrocks-controller/store/keys"
)

type ttlSetter interface {
	SetWithTTL(ctx context.Context, key string, value []byte, ttlSeconds int64) error
}

type Detector struct {
	store      *store.ClusterStore
	failCfg    *config.FailOverConfig
	multiCfg   *config.MultiDetectConfig
	sessionID  string
	ttlSeconds int64

	mu      sync.Mutex
	counts  map[string]map[string]int
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
}

func NewDetector(stor *store.ClusterStore, ctrlCfg *config.ControllerConfig, sessionID string) *Detector {
	ttl := int64((ctrlCfg.MultiDetect.WindowPeriods + 1) * ctrlCfg.FailOver.PingIntervalSeconds)
	return &Detector{
		store:      stor,
		failCfg:    ctrlCfg.FailOver,
		multiCfg:   ctrlCfg.MultiDetect,
		sessionID:  sessionID,
		ttlSeconds: ttl,
		counts:     make(map[string]map[string]int),
	}
}

func (d *Detector) StartAllClusters(ctx context.Context) error {
	if !d.multiCfg.Enabled {
		return nil
	}
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return nil
	}
	d.started = true
	d.mu.Unlock()

	runCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.wg.Add(1)
	go d.loop(runCtx)
	return nil
}

func (d *Detector) Stop(context.Context) error {
	d.mu.Lock()
	if !d.started {
		d.mu.Unlock()
		return nil
	}
	cancel := d.cancel
	d.mu.Unlock()

	cancel()
	d.wg.Wait()
	return nil
}

func (d *Detector) loop(ctx context.Context) {
	defer d.wg.Done()
	interval := time.Duration(d.failCfg.PingIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.detectOnce(ctx)
	for {
		select {
		case <-ticker.C:
			d.detectOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (d *Detector) detectOnce(ctx context.Context) {
	namespaces, err := d.store.ListNamespace(ctx)
	if err != nil {
		logger.Get().With(zap.Error(err)).Warn("Detector failed to list namespaces")
		return
	}
	for _, ns := range namespaces {
		clusters, err := d.store.ListCluster(ctx, ns)
		if err != nil {
			logger.Get().With(zap.Error(err), zap.String("namespace", ns)).Warn("Detector failed to list clusters")
			continue
		}
		for _, clusterName := range clusters {
			clusterInfo, err := d.store.GetCluster(ctx, ns, clusterName)
			if err != nil {
				logger.Get().With(
					zap.Error(err),
					zap.String("namespace", ns),
					zap.String("cluster", clusterName),
				).Warn("Detector failed to get cluster snapshot")
				continue
			}
			d.evaluateCluster(ctx, ns, clusterInfo)
		}
	}
}

func (d *Detector) evaluateCluster(ctx context.Context, namespace string, clusterInfo *store.Cluster) {
	clusterKey := namespace + "/" + clusterInfo.Name
	for _, shard := range clusterInfo.Shards {
		for _, node := range shard.Nodes {
			nodeID := node.ID()
			statusErr := probeNode(ctx, node)
			count := d.updateCounter(clusterKey, nodeID, statusErr == nil)
			if err := d.writeRecord(ctx, namespace, clusterInfo.Name, nodeID, count); err != nil {
				logger.Get().With(
					zap.Error(err),
					zap.String("namespace", namespace),
					zap.String("cluster", clusterInfo.Name),
					zap.String("node", nodeID),
				).Warn("Detector failed to persist record")
			}
			if statusErr != nil {
				metrics.MultiDetectDetectWrites.WithLabelValues("fail").Inc()
			} else {
				metrics.MultiDetectDetectWrites.WithLabelValues("pass").Inc()
			}
		}
	}
}

func (d *Detector) updateCounter(clusterKey, nodeID string, pass bool) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	counts, ok := d.counts[clusterKey]
	if !ok {
		counts = make(map[string]int)
		d.counts[clusterKey] = counts
	}
	if pass {
		counts[nodeID] = 0
		return 0
	}
	next := counts[nodeID] + 1
	if next > int(d.failCfg.MaxPingCount) {
		next = int(d.failCfg.MaxPingCount)
	}
	counts[nodeID] = next
	return next
}

func (d *Detector) writeRecord(ctx context.Context, namespace, cluster, nodeID string, count int) error {
	record := keys.DetectRecord{Count: count, TS: time.Now().Unix()}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	sessionKey := keys.DetectionSessionPrefix(d.multiCfg.KeyPrefix, namespace, cluster, d.sessionID)
	if err := d.setWithTTL(ctx, sessionKey, []byte{}, d.ttlSeconds); err != nil {
		return err
	}

	key := keys.DetectionKey(d.multiCfg.KeyPrefix, namespace, cluster, d.sessionID, nodeID)
	return d.setWithTTL(ctx, key, payload, d.ttlSeconds)
}

func (d *Detector) setWithTTL(ctx context.Context, key string, value []byte, ttl int64) error {
	engine := d.store.GetEngine()
	if setter, ok := engine.(ttlSetter); ok {
		return setter.SetWithTTL(ctx, key, value, ttl)
	}
	return engine.Set(ctx, key, value)
}

func probeNode(ctx context.Context, node store.Node) error {
	_, err := node.GetClusterInfo(ctx)
	return err
}
