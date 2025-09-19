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
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/apache/kvrocks-controller/store/engine"
)

const defaultBatchSize = 256

// ScanAll collects key/value pairs under the prefix by listing sessions and batching node reads.
// It avoids hot-path per-key roundtrips by grouping node fetches per session and executing them in parallel batches.
func ScanAll(ctx context.Context, eng engine.Engine, prefix string, batchSize int) (map[string][]byte, error) {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	sessionEntries, err := eng.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	results := make(map[string][]byte)
	var mu sync.Mutex
	trimmedPrefix := strings.TrimRight(prefix, "/")
	for start := 0; start < len(sessionEntries); start += batchSize {
		end := start + batchSize
		if end > len(sessionEntries) {
			end = len(sessionEntries)
		}
		batch := sessionEntries[start:end]
		if len(batch) == 0 {
			continue
		}
		if err := gatherSessionBatch(ctx, eng, trimmedPrefix, batch, &mu, results); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func gatherSessionBatch(ctx context.Context, eng engine.Engine, basePrefix string, batch []engine.Entry, mu *sync.Mutex, results map[string][]byte) error {
	eg, ctx := errgroup.WithContext(ctx)
	for _, entry := range batch {
		entry := entry
		eg.Go(func() error {
			sessionPrefix := fmt.Sprintf("%s/%s", basePrefix, entry.Key)
			nodeEntries, err := eng.List(ctx, sessionPrefix)
			if err != nil {
				return err
			}
			if len(nodeEntries) == 0 {
				return nil
			}
			sessionPrefix = strings.TrimRight(sessionPrefix, "/")
			local := make(map[string][]byte, len(nodeEntries))
			for _, nodeEntry := range nodeEntries {
				fullKey := fmt.Sprintf("%s/%s", sessionPrefix, nodeEntry.Key)
				valueCopy := make([]byte, len(nodeEntry.Value))
				copy(valueCopy, nodeEntry.Value)
				local[fullKey] = valueCopy
			}
			mu.Lock()
			for k, v := range local {
				results[k] = v
			}
			mu.Unlock()
			return nil
		})
	}
	return eg.Wait()
}
