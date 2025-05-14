package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Target struct {
	HTTP []Http `yaml:"http"`
	DNS  []Dns  `yaml:"dns"`
	ICMP []Icmp `yaml:"icmp"`
	K8S  K8s    `yaml:"k8s"`
}

type Http struct {
	Name              string `yaml:"name"`
	URL               string `yaml:"url"`
	RPS               int    `yaml:"rps"`
	Timeout           int    `yaml:"timeout"`
	TLSSkipVerify     bool   `yaml:"tls_skip_verify"`
	DisableKeepAlives bool   `yaml:"disable_keep_alives"`
	H2cEnabled        bool   `yaml:"h2c_enabled"`
	Host              string `yaml:"host"`
}

type Dns struct {
	Name     string `yaml:"name"`
	Server   string `yaml:"server"`
	Domain   string `yaml:"domain"`
	Interval int    `yaml:"interval"`
}

type Icmp struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Interval int    `yaml:"interval"`
}

type K8s struct {
	Enabled       bool   `yaml:"enabled"`
	Namespace     string `yaml:"namespace"`
	LabelSelector string `yaml:"label_selector"`
	Interval      int    `yaml:"interval"`
}

type Config struct {
	Listen  string `yaml:"listen"`
	Targets Target `yaml:"targets"`
}

var cfg Config

func Read(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &cfg)
}

func Get() Config {
	return cfg
}
