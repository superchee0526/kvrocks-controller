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

package keys

import "fmt"

const detectionSubPath = "detection"

// DetectionKey builds the storage key for controller detection results.
func DetectionKey(prefix, namespace, cluster, sessionID, nodeID string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s", trimTrailingSlash(prefix), namespace, cluster, detectionSubPath, sessionID, nodeID)
}

// DetectionSessionPrefix returns the prefix that groups records per controller session.
func DetectionSessionPrefix(prefix, namespace, cluster, sessionID string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", trimTrailingSlash(prefix), namespace, cluster, detectionSubPath, sessionID)
}

// DetectionClusterPrefix returns the prefix for all detection records of a cluster.
func DetectionClusterPrefix(prefix, namespace, cluster string) string {
	return fmt.Sprintf("%s/%s/%s/%s", trimTrailingSlash(prefix), namespace, cluster, detectionSubPath)
}

func trimTrailingSlash(prefix string) string {
	if len(prefix) == 0 {
		return prefix
	}
	if prefix[len(prefix)-1] == '/' {
		return prefix[:len(prefix)-1]
	}
	return prefix
}

// DetectRecord captures a controller's current view for a node within the detection window.
type DetectRecord struct {
	Count int   `json:"count"`
	TS    int64 `json:"ts"`
}
