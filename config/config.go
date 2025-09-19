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
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/apache/kvrocks-controller/store/engine/consul"
	"github.com/apache/kvrocks-controller/store/engine/raft"

	"github.com/go-playground/validator/v10"

	"github.com/apache/kvrocks-controller/logger"
	"github.com/apache/kvrocks-controller/store/engine/etcd"
	"github.com/apache/kvrocks-controller/store/engine/zookeeper"
)

type AdminConfig struct {
	Addr string `yaml:"addr"`
}

type FailOverConfig struct {
	PingIntervalSeconds int   `yaml:"ping_interval_seconds"`
	MaxPingCount        int64 `yaml:"max_ping_count"`
}

type ControllerConfig struct {
	FailOver    *FailOverConfig    `yaml:"failover"`
	MultiDetect *MultiDetectConfig `yaml:"multi_detect"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	Filename   string `yaml:"filename"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	MaxSize    int    `yaml:"max_size"`
	Compress   bool   `yaml:"compress"`
}

const defaultPort = 9379

type Config struct {
	Addr        string            `yaml:"addr"`
	StorageType string            `yaml:"storage_type"`
	Etcd        *etcd.Config      `yaml:"etcd"`
	Zookeeper   *zookeeper.Config `yaml:"zookeeper"`
	Raft        *raft.Config      `yaml:"raft"`
	Consul      *consul.Config    `yaml:"consul"`
	Admin       AdminConfig       `yaml:"admin"`
	Controller  *ControllerConfig `yaml:"controller"`
	Log         *LogConfig        `yaml:"log"`
}

func DefaultFailOverConfig() *FailOverConfig {
	return &FailOverConfig{
		PingIntervalSeconds: 3,
		MaxPingCount:        5,
	}
}

func DefaultMultiDetectConfig() *MultiDetectConfig {
	return &MultiDetectConfig{
		Enabled:       false,
		WindowPeriods: 3,
		Quorum: QuorumConfig{
			Mode:     QuorumModeMajority,
			MinVotes: 0,
		},
		KeyPrefix:                defaultNamespacePrefix,
		RequireQuorumOnPromotion: true,
		AggregateIntervalMS:      500,
	}
}

func Default() *Config {
	c := &Config{
		Etcd: &etcd.Config{
			Addrs: []string{"127.0.0.1:2379"},
		},
		Controller: &ControllerConfig{
			FailOver:    DefaultFailOverConfig(),
			MultiDetect: DefaultMultiDetectConfig(),
		},
	}
	c.Addr = c.getAddr()
	return c
}

func (c *Config) Validate() error {
	if c.Controller.FailOver.MaxPingCount < 3 {
		return errors.New("max ping count required >= 3")
	}
	if c.Controller.FailOver.PingIntervalSeconds < 1 {
		return errors.New("ping interval required >= 1s")
	}
	if c.Controller.MultiDetect == nil {
		c.Controller.MultiDetect = DefaultMultiDetectConfig()
	} else {
		c.Controller.MultiDetect.fillDefaults()
		if err := c.Controller.MultiDetect.Validate(); err != nil {
			return err
		}
	}
	hostPort := strings.Split(c.Addr, ":")
	if hostPort[0] == "0.0.0.0" || hostPort[0] == "127.0.0.1" {
		logger.Get().Warn("Leader forward may not work if the host is " + hostPort[0])
	}
	return nil
}

func (c *Config) getAddr() string {
	// env has higher priority than configuration.
	// case: get addr from env
	checker := validator.New()
	host := os.Getenv("KVROCKS_CONTROLLER_HTTP_HOST")
	port := os.Getenv("KVROCKS_CONTROLLER_HTTP_PORT")
	addr := host + ":" + port
	err := checker.Var(addr, "required,tcp_addr")
	if err == nil {
		return fmt.Sprintf("%s:%s", host, port)
	}
	if c.Addr != "" {
		return c.Addr
	}

	// case: addr is empty
	ip := getLocalIP()
	if ip != "" {
		return fmt.Sprintf("%s:%d", ip, defaultPort)
	}
	return fmt.Sprintf("127.0.0.1:%d", defaultPort)
}

// getLocalIP returns the non loopback local IP of the host.
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		ipnet, ok := address.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
