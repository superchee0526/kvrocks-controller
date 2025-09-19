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
package consul

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/apache/kvrocks-controller/util"
	"github.com/stretchr/testify/require"
)

var addr = func() string {
	if v := os.Getenv("KVROCKS_CONSUL_ADDR"); v != "" {
		return v
	}
	return "127.0.0.1:8500"
}()

func TestBasicOperations(t *testing.T) {
	id := util.RandString(40)
	testElectPath := util.RandString(32)
	print(testElectPath)
	persist, err := New(id, &Config{
		ElectPath: testElectPath,
		Addrs:     []string{addr},
	})
	require.NoError(t, err)
	defer persist.Close()
	go func() {
		for range persist.LeaderChange() {
			// do nothing
		}
	}()

	ctx := context.Background()
	keys := []string{"/a/b/c0", "/a/b/c1", "/a/b/c2"}
	value := []byte("v")
	for _, key := range keys {
		require.NoError(t, persist.Set(ctx, key, value))
		gotValue, err := persist.Get(ctx, key)
		require.NoError(t, err)
		require.Equal(t, value, gotValue)
	}
	entries, err := persist.List(ctx, "/a/b")
	require.NoError(t, err)
	require.Equal(t, len(keys), len(entries))
	for _, key := range keys {
		require.NoError(t, persist.Delete(ctx, key))
	}
}

func TestElect(t *testing.T) {
	endpoints := []string{addr}

	testElectPath := util.RandString(32)
	id0 := util.RandString(40)
	node0, err := New(id0, &Config{
		ElectPath: testElectPath,
		Addrs:     endpoints,
	})
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		return node0.Leader() == node0.myID
	}, 10*time.Second, 100*time.Millisecond, "node0 should be the leader")

	id1 := util.RandString(40)
	node1, err := New(id1, &Config{
		ElectPath: testElectPath,
		Addrs:     endpoints,
	})
	require.NoError(t, err)
	require.Eventuallyf(t, func() bool {
		return node1.Leader() == node0.myID
	}, 10*time.Second, 100*time.Millisecond, "node1's leader should be the node0")

	go func() {
		for {
			select {
			case <-node0.LeaderChange():
				// do nothing
			case <-node1.LeaderChange():
				// do nothing
			}
		}
	}()

	require.NoError(t, node0.Close())

	require.Eventuallyf(t, func() bool {
		return node1.Leader() == node1.myID
	}, 25*time.Second, 100*time.Millisecond, "node1 should be the leader")
}
