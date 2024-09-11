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
	BaseAMIPattern       = `Found\sImage\sID:\s+(ami-[a-zA-Z0-9]+)`
	LastestAMIPattern    = `AMI:\s+(ami-[a-zA-Z0-9]+)`
	SecurityGroupPattern = `Found\ssecurity\sgroup\s(sg-[a-f0-9]{17})`
	KeyPairPattern       = `Creating\stemporary\skeypair:\s(packer_[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`
	WorkingDir           = "ami-builder"
	WorkingDirRoot       = "workspace"
)

type EntryKey string

// Config struct with a map[Entry]interface{}
type Config struct {
	Values map[EntryKey]interface{}
}

// NewConfig initializes the Config struct with an empty map
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
		// YAML successfully loaded
		return c.loadFromEnvVariables()
	}

	// Attempt to load as JSON
	if err := c.loadFromJSON(data); err == nil {
		// JSON successfully loaded
		return c.loadFromEnvVariables()
	}

	// Attempt to load as .env file
	if err := c.loadFromDotEnv(filePath); err == nil {
		// .env successfully loaded
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
