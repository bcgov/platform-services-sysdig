package helpers

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config holds everything from your local-test.yaml
type Config struct {
	SysdigAPIEndpoint string `yaml:"sysdig_api_endpoint"`
	SysdigToken       string `yaml:"sysdig_token"`

	SysdigTeamSA struct {
		Name string `yaml:"name"`
		Role string `yaml:"role"`
	} `yaml:"sysdig_team_sa"`

	Team struct {
		Description string `yaml:"description"`
		Users       []struct {
			Name string `yaml:"name"`
			Role string `yaml:"role"`
		} `yaml:"users"`
	} `yaml:"team"`
}

// LoadConfig reads and parses the given YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
