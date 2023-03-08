/*
 * Copyright 2021 OpsMx, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

const (
	defaultCACertPath     = "/app/config/ca.pem"
	defaultUserconfigPath = "/app/config/services.yaml"
	defaultAuthTokenPath  = "/app/secrets/authtoken"
	defaultDialMaxRetries = 10
	defaultDialRetryTime  = 10
)

// agentConfig holds all the configuration for the agent.  The
// configuration file is loaded from disk first, and then any
// environment variables are applied.
type agentConfig struct {
	ControllerHostname string `json:"controllerHostname,omitempty" yaml:"controllerHostname,omitempty"`
	CACertPath         string `json:"caCertPath,omitempty" yaml:"caCertPath,omitempty"`
	AuthTokenPath      string `json:"authTokenPath,omitempty" yaml:"authTokenPath,omitempty"`
	ServicesConfigPath string `json:"servicesConfigPath,omitempty" yaml:"servicesConfigPath,omitempty"`
	DialMaxRetries     int    `json:"dialMaxRetries,omitempty" yaml:"dialMaxRetries,omitempty"`
	DialRetryTime      int    `json:"dialRetryTime,omitempty" yaml:"dialRetryTime,omitempty"`
}

func (c *agentConfig) applyDefaults() {
	if len(c.ControllerHostname) == 0 {
		c.ControllerHostname = "agent-controller:9001"
	}

	if len(c.CACertPath) == 0 {
		c.CACertPath = defaultCACertPath
	}

	if len(c.AuthTokenPath) == 0 {
		c.AuthTokenPath = defaultAuthTokenPath
	}

	if len(c.ServicesConfigPath) == 0 {
		c.ServicesConfigPath = defaultUserconfigPath
	}

	if c.DialMaxRetries == 0 {
		c.DialMaxRetries = defaultDialMaxRetries
	}

	if c.DialRetryTime == 0 {
		c.DialRetryTime = defaultDialRetryTime
	}
}

// loadConfig will load YAML configuration from the provided filename, and then apply
// environment variables to override some subset of available options.
func loadConfig(filename string) (*agentConfig, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &agentConfig{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, err
	}

	config.applyDefaults()

	return config, nil
}
