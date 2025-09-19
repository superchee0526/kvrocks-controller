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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/apache/kvrocks-controller/config"
	"github.com/apache/kvrocks-controller/store"
	"github.com/apache/kvrocks-controller/store/engine"
	"github.com/apache/kvrocks-controller/store/keys"
)

type stubTTLEngine struct {
	mu     sync.Mutex
	writes map[string]ttlWrite
}

type ttlWrite struct {
	value []byte
	ttl   int64
}

func newStubTTLEngine() *stubTTLEngine {
	return &stubTTLEngine{writes: make(map[string]ttlWrite)}
}

func (s *stubTTLEngine) SetWithTTL(_ context.Context, key string, value []byte, ttl int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes[key] = ttlWrite{value: append([]byte(nil), value...), ttl: ttl}
	return nil
}

func (s *stubTTLEngine) Set(ctx context.Context, key string, value []byte) error {
	return s.SetWithTTL(ctx, key, value, 0)
}

// Unused methods for the tests.
func (s *stubTTLEngine) Get(context.Context, string) ([]byte, error)          { return nil, nil }
func (s *stubTTLEngine) Exists(context.Context, string) (bool, error)         { return false, nil }
func (s *stubTTLEngine) Delete(context.Context, string) error                 { delete(s.writes, ""); return nil }
func (s *stubTTLEngine) List(context.Context, string) ([]engine.Entry, error) { return nil, nil }
func (s *stubTTLEngine) Close() error                                         { return nil }
func (s *stubTTLEngine) ID() string                                           { return "stub" }
func (s *stubTTLEngine) Leader() string                                       { return "stub" }
func (s *stubTTLEngine) LeaderChange() <-chan bool                            { return make(chan bool) }
func (s *stubTTLEngine) IsReady(context.Context) bool                         { return true }

func TestDetectorUpdateCounter(t *testing.T) {
	eng := newStubTTLEngine()
	store := store.NewClusterStore(eng)

	cfg := &config.ControllerConfig{
		FailOver:    &config.FailOverConfig{PingIntervalSeconds: 2, MaxPingCount: 3},
		MultiDetect: &config.MultiDetectConfig{Enabled: true, WindowPeriods: 3, Quorum: config.QuorumConfig{Mode: config.QuorumModeMajority}, KeyPrefix: "/kvrocks/metadata"},
	}

	detector := NewDetector(store, cfg, "session-1")

	count := detector.updateCounter("ns/cluster", "node-1", false)
	require.Equal(t, 1, count)

	count = detector.updateCounter("ns/cluster", "node-1", false)
	require.Equal(t, 2, count)

	count = detector.updateCounter("ns/cluster", "node-1", true)
	require.Equal(t, 0, count)

	detector.stop()
}

func TestDetectorWriteRecordUsesTTL(t *testing.T) {
	eng := newStubTTLEngine()
	store := store.NewClusterStore(eng)
	cfg := &config.ControllerConfig{
		FailOver:    &config.FailOverConfig{PingIntervalSeconds: 2, MaxPingCount: 3},
		MultiDetect: &config.MultiDetectConfig{Enabled: true, WindowPeriods: 3, Quorum: config.QuorumConfig{Mode: config.QuorumModeMajority}, KeyPrefix: "/kvrocks/metadata"},
	}

	detector := NewDetector(store, cfg, "session-xyz")

	err := detector.writeRecord(context.Background(), "ns", "cluster", "node-3", 2)
	require.NoError(t, err)

	expectedTTL := int64((cfg.MultiDetect.WindowPeriods + 1) * cfg.FailOver.PingIntervalSeconds)

	eng.mu.Lock()
	defer eng.mu.Unlock()

	sessionKey := keys.DetectionSessionPrefix(cfg.MultiDetect.KeyPrefix, "ns", "cluster", "session-xyz")
	recordKey := keys.DetectionKey(cfg.MultiDetect.KeyPrefix, "ns", "cluster", "session-xyz", "node-3")

	require.Contains(t, eng.writes, sessionKey)
	require.Equal(t, expectedTTL, eng.writes[sessionKey].ttl)

	require.Contains(t, eng.writes, recordKey)
	require.Equal(t, expectedTTL, eng.writes[recordKey].ttl)

	var record keys.DetectRecord
	require.NoError(t, json.Unmarshal(eng.writes[recordKey].value, &record))
	require.Equal(t, 2, record.Count)
	require.True(t, time.Since(time.Unix(record.TS, 0)) < 2*time.Second)
}

func (d *Detector) stop() {
	if d.cancel != nil {
		d.cancel()
	}
}
