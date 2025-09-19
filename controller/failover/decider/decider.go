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
	"github.com/apache/kvrocks-controller/metrics"
	"github.com/apache/kvrocks-controller/store/keys"
)

type Decision string

const (
	DownByQuorum Decision = "down_by_quorum"
	UpByQuorum   Decision = "up_by_quorum"
	UpBySelf     Decision = "up_by_self"
)

type RecordReader interface {
	GetFreshRecords(clusterKey, nodeID string) []keys.DetectRecord
}

func JudgeNodeStateQuorum(reader RecordReader, clusterKey, nodeID string, maxFail int64) Decision {
	records := reader.GetFreshRecords(clusterKey, nodeID)
	decision := UpBySelf
	failVotes, passVotes := tallyVotes(records, maxFail)
	switch {
	case failVotes > passVotes:
		decision = DownByQuorum
	case passVotes > failVotes:
		decision = UpByQuorum
	default:
		decision = UpBySelf
	}
	metrics.MultiDetectDecisionTotal.WithLabelValues(string(decision)).Inc()
	return decision
}

func HasMajorityPass(reader RecordReader, clusterKey, nodeID string, maxFail int64) bool {
	records := reader.GetFreshRecords(clusterKey, nodeID)
	failVotes, passVotes := tallyVotes(records, maxFail)
	return passVotes > failVotes
}

func tallyVotes(records []keys.DetectRecord, maxFail int64) (failVotes, passVotes int) {
	for _, r := range records {
		if int64(r.Count) >= maxFail {
			failVotes++
		} else {
			passVotes++
		}
	}
	return failVotes, passVotes
}
