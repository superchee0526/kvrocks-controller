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

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	MultiDetectEnabledGauge       prometheus.Gauge
	MultiDetectDetectWrites       *prometheus.CounterVec
	MultiDetectQuorumSessions     *prometheus.GaugeVec
	MultiDetectDecisionTotal      *prometheus.CounterVec
	MultiDetectPromotionBlocked   *prometheus.CounterVec
	MultiDetectAggregatorDuration prometheus.ObserverVec
	MultiDetectAggregatorRecords  prometheus.Counter
)

func initMultiDetectMetrics() {
	MultiDetectEnabledGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_multidetect_enabled",
		Help:      "Indicates whether multi-detect quorum mode is enabled (1=yes)",
	})
	prometheus.MustRegister(MultiDetectEnabledGauge)

	MultiDetectDetectWrites = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_detect_write_total",
		Help:      "Total detector writes grouped by status",
	}, []string{"status"})
	prometheus.MustRegister(MultiDetectDetectWrites)

	MultiDetectQuorumSessions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_quorum_sessions_in_window",
		Help:      "Number of controller sessions contributing recent data per node",
	}, []string{"namespace", "cluster", "node"})
	prometheus.MustRegister(MultiDetectQuorumSessions)

	MultiDetectDecisionTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_decision_total",
		Help:      "Failover decisions derived from quorum evaluation",
	}, []string{"decision"})
	prometheus.MustRegister(MultiDetectDecisionTotal)

	MultiDetectPromotionBlocked = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_promotion_blocked_total",
		Help:      "Number of promotions blocked due to quorum requirements",
	}, []string{"reason"})
	prometheus.MustRegister(MultiDetectPromotionBlocked)

	MultiDetectAggregatorDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_aggregator_scan_duration_ms",
		Help:      "Duration of aggregator scans in milliseconds",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
	}, []string{"phase"})
	prometheus.MustRegister(MultiDetectAggregatorDuration)

	MultiDetectAggregatorRecords = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: _namespace,
		Subsystem: _subsystem,
		Name:      "kvctl_aggregator_records_loaded",
		Help:      "Total number of detection records loaded by the aggregator",
	})
	prometheus.MustRegister(MultiDetectAggregatorRecords)
}

func init() {
	initMultiDetectMetrics()
}
