/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

type regexPattern string

const (
	BaseAMIPattern        regexPattern = `Found\sImage\sID:\s+(ami-[a-zA-Z0-9]+)`
	JobServiceAccountName              = "packer-ami-builder"
	KeyPairPattern        regexPattern = `Creating\stemporary\skeypair:\s(packer_[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`
	LastestAMIPattern     regexPattern = `AMI:\s+(ami-[a-zA-Z0-9]+)`
	SecurityGroupPattern  regexPattern = `Found\ssecurity\sgroup\s(sg-[a-f0-9]{17})`
	WorkingDir                         = "ami-builder"
	WorkingDirRoot                     = "workspace"
	Finalizer                          = "ami.opsy.dev/finalizer"
)

type EntryKey string

type Config struct {
	Values map[EntryKey]interface{}
}

func NewConfig() *Config {
	return &Config{
		Values: make(map[EntryKey]interface{}),
	}
}

func (c *Config) loadFromYAML(data []byte) error {
	tempMap := make(map[string]interface{})
	err := yaml.Unmarshal(data, &tempMap)
	if err != nil {
		return err
	}
	for key, value := range tempMap {
		c.Values[EntryKey(key)] = value
	}
	return nil
}

func (c *Config) loadFromJSON(data []byte) error {
	tempMap := make(map[string]interface{})
	err := json.Unmarshal(data, &tempMap)
	if err != nil {
		return err
	}
	for key, value := range tempMap {
		c.Values[EntryKey(key)] = value
	}
	return nil
}

func (c *Config) loadFromDotEnv(filePath string) error {
	err := godotenv.Load(filePath)
	if err != nil {
		return err
	}
	return c.loadFromEnvVariables()
}

func (c *Config) loadFromEnvVariables() error {
	for key := range c.Values {
		envValue := os.Getenv(string(key))
		if envValue != "" {
			c.Values[EntryKey(key)] = envValue
		}
	}
	return nil
}

func (c *Config) LoadConfig(filePath string) error {
	if filePath == "" {
		// No file provided, load only from environment variables
		return c.loadFromEnvVariables()
	}

	// Read the file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read the config file: %v", err)
	}

	// Attempt to load as YAML
	if err := c.loadFromYAML(data); err == nil {
		return c.loadFromEnvVariables()
	}

	// Attempt to load as JSON
	if err := c.loadFromJSON(data); err == nil {
		return c.loadFromEnvVariables()
	}

	// Attempt to load as .env file
	if err := c.loadFromDotEnv(filePath); err == nil {
		return c.loadFromEnvVariables()
	}

	// If none of the formats matched, return an error
	return fmt.Errorf("failed to determine the file format or load the configuration")
}

func (c *Config) Get(key EntryKey) (interface{}, error) {
	value, exists := c.Values[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

func (c *Config) Set(key EntryKey, value interface{}) {
	c.Values[key] = value
}
