package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var config Config

func Get() Config {
	return config
}

func Read(path string) error {
	viper.SetConfigName(path)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		return fmt.Errorf("cannot read config file: %w", err)
	}

	err = viper.Unmarshal(&config)

	if err != nil {
		return fmt.Errorf("cannot unmarshal config file: %w", err)
	}

	return nil
}

type Config struct {
	Listen  string `mapstructure:"listen"`
	Targets Target `mapstructure:"targets"`
}

type Target struct {
	HTTP []HTTP `mapstructure:"http"`
	DNS  []DNS  `mapstructure:"dns"`
	K8S  K8S    `mapstructure:"k8s"`
}

type HTTP struct {
	Name              string        `mapstructure:"name"`
	URL               string        `mapstructure:"url"`
	RPS               float64       `mapstructure:"rps"`
	Timeout           time.Duration `mapstructure:"timeout"`
	TLSSkipVerify     bool          `mapstructure:"tls_skip_verify"`
	DisableKeepAlives bool          `mapstructure:"disable_keepalives"`
	Host              string        `mapstructure:"host"`
}

type DNS struct {
	Name       string        `mapstructure:"name"`
	Domain     string        `mapstructure:"domain"`
	RecordType string        `mapstructure:"record_type"`
	RPS        float64       `mapstructure:"rps"`
	ServerIP   string        `mapstructure:"server_ip"`
	ServerPort int           `mapstructure:"server_port"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type K8S struct {
	Enabled     bool              `mapstructure:"enabled"`
	SimpleProbe []K8S_SimpleProbe `mapstructure:"simple-probe"`
}

type K8S_SimpleProbe struct {
	NameSpace string  `mapstructure:"namespace"`
	RPS       float64 `mapstructure:"rps"`
}
