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

package scanhelper

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/apache/kvrocks-controller/store/engine"
)

type fakeEngine struct {
	mu        sync.Mutex
	data      map[string][]byte
	listCalls int
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{data: make(map[string][]byte)}
}

func (f *fakeEngine) insert(key string, value []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = value
}

func (f *fakeEngine) List(_ context.Context, prefix string) ([]engine.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++

	trimmedPrefix := strings.TrimRight(prefix, "/")
	if trimmedPrefix == "" {
		trimmedPrefix = "/"
	}

	dedup := make(map[string]struct{})
	var entries []engine.Entry
	for key, value := range f.data {
		if !strings.HasPrefix(key, trimmedPrefix) {
			continue
		}
		suffix := strings.TrimPrefix(key, trimmedPrefix)
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

// Unused engine.Engine methods (minimal stub for tests).
func (f *fakeEngine) Get(context.Context, string) ([]byte, error)  { panic("not implemented") }
func (f *fakeEngine) Exists(context.Context, string) (bool, error) { panic("not implemented") }
func (f *fakeEngine) Set(context.Context, string, []byte) error    { panic("not implemented") }
func (f *fakeEngine) Delete(context.Context, string) error         { panic("not implemented") }
func (f *fakeEngine) Close() error                                 { return nil }
func (f *fakeEngine) ID() string                                   { return "fake" }
func (f *fakeEngine) Leader() string                               { return "fake" }
func (f *fakeEngine) LeaderChange() <-chan bool                    { return make(chan bool) }
func (f *fakeEngine) IsReady(context.Context) bool                 { return true }

func TestScanAllBatchedSessions(t *testing.T) {
	eng := newFakeEngine()
	prefix := "/kvrocks/metadata/ns/cluster/detection"
	eng.insert("/kvrocks/metadata/ns/cluster/detection/sessionA/node-1", []byte("payload-A1"))
	eng.insert("/kvrocks/metadata/ns/cluster/detection/sessionA/node-2", []byte("payload-A2"))
	eng.insert("/kvrocks/metadata/ns/cluster/detection/sessionB/node-3", []byte("payload-B3"))

	ctx := context.Background()
	res, err := ScanAll(ctx, eng, prefix, 1)
	require.NoError(t, err)
	require.Len(t, res, 3)
	require.Equal(t, []byte("payload-A1"), res["/kvrocks/metadata/ns/cluster/detection/sessionA/node-1"])
	require.Equal(t, []byte("payload-A2"), res["/kvrocks/metadata/ns/cluster/detection/sessionA/node-2"])
	require.Equal(t, []byte("payload-B3"), res["/kvrocks/metadata/ns/cluster/detection/sessionB/node-3"])

	// one list for prefix + one per session (batch size 1 => sequential batches)
	require.Equal(t, 3, eng.listCalls)
}
