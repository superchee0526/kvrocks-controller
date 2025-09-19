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
	"errors"
	"strings"
)

const (
	QuorumModeMajority = "majority"
	QuorumModeFixed    = "fixed"

	defaultNamespacePrefix = "/kvrocks/metadata"
)

type QuorumConfig struct {
	Mode     string `yaml:"mode"`
	MinVotes int    `yaml:"min_votes"`
}

type MultiDetectConfig struct {
	Enabled                  bool         `yaml:"enabled"`
	WindowPeriods            int          `yaml:"window_periods"`
	Quorum                   QuorumConfig `yaml:"quorum"`
	KeyPrefix                string       `yaml:"key_prefix"`
	RequireQuorumOnPromotion bool         `yaml:"require_quorum_on_promotion"`
	AggregateIntervalMS      int          `yaml:"aggregate_interval_ms"`
}

func (c *MultiDetectConfig) fillDefaults() {
	if c.WindowPeriods <= 0 {
		c.WindowPeriods = 3
	}
	c.Quorum.Mode = strings.ToLower(strings.TrimSpace(c.Quorum.Mode))
	if c.Quorum.Mode == "" {
		c.Quorum.Mode = QuorumModeMajority
	}
	if c.Quorum.MinVotes < 0 {
		c.Quorum.MinVotes = 0
	}
	if c.KeyPrefix == "" {
		c.KeyPrefix = defaultNamespacePrefix
	}
	if c.AggregateIntervalMS <= 0 {
		c.AggregateIntervalMS = 500
	}
	if !c.RequireQuorumOnPromotion {
		c.RequireQuorumOnPromotion = true
	}
}

func (c *MultiDetectConfig) Validate() error {
	switch c.Quorum.Mode {
	case QuorumModeMajority, QuorumModeFixed:
	default:
		return errors.New("multi-detect quorum mode must be 'majority' or 'fixed'")
	}
	if c.WindowPeriods <= 0 {
		return errors.New("multi-detect window_periods must be > 0")
	}
	if c.AggregateIntervalMS <= 0 {
		return errors.New("multi-detect aggregate_interval_ms must be > 0")
	}
	if c.KeyPrefix == "" {
		return errors.New("multi-detect key_prefix must not be empty")
	}
	return nil
}
