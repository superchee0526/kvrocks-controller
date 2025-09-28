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

package api

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/apache/kvrocks-controller/consts"
	"github.com/apache/kvrocks-controller/server/helper"
	"github.com/apache/kvrocks-controller/store"
)

type MigrateSlotRequest struct {
	Target   int             `json:"target" validate:"required"`
	Slot     store.SlotRange `json:"slot" validate:"required"` // we don't use store.MigratingSlot here because we expect a valid SlotRange
	SlotOnly bool            `json:"slot_only"`
}

type CreateClusterRequest struct {
	Name     string   `json:"name" validate:"required"`
	Nodes    []string `json:"nodes" validate:"required"`
	Password string   `json:"password"`
	Replicas int      `json:"replicas"`
}

type ClusterHandler struct {
	s     store.Store
	locks sync.Map
}

func (handler *ClusterHandler) getLock(ns, cluster string) *sync.RWMutex {
	value, _ := handler.locks.LoadOrStore(fmt.Sprintf("%s/%s", ns, cluster), &sync.RWMutex{})
	lock, _ := value.(*sync.RWMutex)
	return lock
}

func (handler *ClusterHandler) List(c *gin.Context) {
	namespace := c.Param("namespace")
	clusters, err := handler.s.ListCluster(c, namespace)
	if err != nil && !errors.Is(err, consts.ErrNotFound) {
		helper.ResponseError(c, err)
		return
	}
	helper.ResponseOK(c, gin.H{"clusters": clusters})
}

func (handler *ClusterHandler) Get(c *gin.Context) {
	cluster, _ := c.MustGet(consts.ContextKeyCluster).(*store.Cluster)
	helper.ResponseOK(c, gin.H{"cluster": cluster})
}

func (handler *ClusterHandler) Create(c *gin.Context) {
	namespace := c.Param("namespace")
	var req CreateClusterRequest
	if err := c.BindJSON(&req); err != nil {
		helper.ResponseBadRequest(c, err)
		return
	}

	clusterStore := handler.s
	if err := clusterStore.CheckNewNodes(c, req.Nodes); err != nil {
		helper.ResponseError(c, err)
		return
	}

	cluster, err := store.NewCluster(req.Name, req.Nodes, req.Replicas)
	if err != nil {
		helper.ResponseBadRequest(c, err)
		return
	}
	cluster.SetPassword(req.Password)
	checkClusterMode := strings.ToLower(c.GetHeader(consts.HeaderDontCheckClusterMode)) == "yes"
	for _, node := range cluster.GetNodes() {
		if !checkClusterMode {
			break
		}
		version, err := node.CheckClusterMode(c)
		if err != nil {
			helper.ResponseError(c, err)
			return
		}
		if version != -1 {
			helper.ResponseBadRequest(c, errors.New("node is already in cluster mode"))
			return
		}
	}

	if err := clusterStore.CreateCluster(c, namespace, cluster); err != nil {
		helper.ResponseError(c, err)
		return
	}
	helper.ResponseCreated(c, gin.H{"cluster": cluster})
}

func (handler *ClusterHandler) Remove(c *gin.Context) {
	namespace := c.Param("namespace")
	cluster := c.Param("cluster")
	err := handler.s.RemoveCluster(c, namespace, cluster)
	if err != nil {
		helper.ResponseError(c, err)
		return
	}
	helper.ResponseNoContent(c)
}

func (handler *ClusterHandler) MigrateSlot(c *gin.Context) {
	namespace := c.Param("namespace")
	clusterName := c.Param("cluster")

	lock := handler.getLock(namespace, clusterName)
	lock.Lock()
	defer lock.Unlock()

	s, _ := c.MustGet(consts.ContextKeyStore).(*store.ClusterStore)
	cluster, err := s.GetCluster(c, namespace, clusterName)
	if err != nil {
		helper.ResponseError(c, err)
		return
	}

	var req MigrateSlotRequest
	if err := c.BindJSON(&req); err != nil {
		helper.ResponseBadRequest(c, err)
		return
	}

	err = cluster.MigrateSlot(c, req.Slot, req.Target, req.SlotOnly)
	if err != nil {
		helper.ResponseError(c, err)
		return
	}

	err = handler.s.UpdateCluster(c, namespace, cluster)
	if err != nil {
		helper.ResponseError(c, err)
		return
	}
	helper.ResponseOK(c, gin.H{"cluster": cluster})
}

func (handler *ClusterHandler) Import(c *gin.Context) {
	namespace := c.Param("namespace")
	clusterName := c.Param("cluster")
	var req struct {
		Nodes    []string `json:"nodes" validate:"required"`
		Password string   `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		helper.ResponseBadRequest(c, err)
		return
	}
	if len(req.Nodes) == 0 {
		helper.ResponseBadRequest(c, errors.New("nodes should NOT be empty"))
		return
	}

	firstNode := store.NewClusterNode(req.Nodes[0], req.Password)
	clusterNodesStr, err := firstNode.GetClusterNodesString(c)
	if err != nil {
		helper.ResponseError(c, err)
		return
	}
	cluster, err := store.ParseCluster(clusterNodesStr)
	if err != nil {
		helper.ResponseError(c, err)
		return
	}
	cluster.SetPassword(req.Password)

	newNodes := make([]string, 0)
	for _, node := range cluster.GetNodes() {
		newNodes = append(newNodes, node.Addr())
	}
	if err := handler.s.CheckNewNodes(c, newNodes); err != nil {
		helper.ResponseError(c, err)
		return
	}

	cluster.Name = clusterName
	if err := handler.s.CreateCluster(c, namespace, cluster); err != nil {
		helper.ResponseError(c, err)
		return
	}
	helper.ResponseOK(c, gin.H{"cluster": cluster})
}
