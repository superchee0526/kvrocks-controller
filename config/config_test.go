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
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultControllerConfigSet(t *testing.T) {
	cfg := Default()
	expectedControllerConfig := &ControllerConfig{
		FailOver: &FailOverConfig{
			PingIntervalSeconds: 3,
			MaxPingCount:        5,
		},
		MultiDetect: &MultiDetectConfig{
			Enabled:       false,
			WindowPeriods: 3,
			Quorum: QuorumConfig{
				Mode:     QuorumModeMajority,
				MinVotes: 0,
			},
			KeyPrefix:                defaultNamespacePrefix,
			RequireQuorumOnPromotion: true,
			AggregateIntervalMS:      500,
		},
	}

	assert.Equal(t, expectedControllerConfig, cfg.Controller)
}

func TestMultiDetectConfigDefaultsAndValidation(t *testing.T) {
	cfg := &MultiDetectConfig{}
	cfg.fillDefaults()
	assert.NoError(t, cfg.Validate())
	assert.Equal(t, 3, cfg.WindowPeriods)
	assert.Equal(t, QuorumModeMajority, cfg.Quorum.Mode)
	assert.Equal(t, 0, cfg.Quorum.MinVotes)
	assert.Equal(t, defaultNamespacePrefix, cfg.KeyPrefix)
	assert.Equal(t, 500, cfg.AggregateIntervalMS)
	assert.True(t, cfg.RequireQuorumOnPromotion)
}
