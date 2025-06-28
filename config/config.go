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
	ICMP []ICMP `mapstructure:"icmp"`
}

type HTTP struct {
	Name              string        `mapstructure:"name"`
	URL               string        `mapstructure:"url"`
	RPS               int           `mapstructure:"rps"`
	Timeout           time.Duration `mapstructure:"timeout"`
	TLSSkipVerify     bool          `mapstructure:"tls_skip_verify"`
	DisableKeepAlives bool          `mapstructure:"disable_keepalives"`
	H2cEnabled        bool          `mapstructure:"h2c_enabled"`
	Host              string        `mapstructure:"host"`
}

type DNS struct {
	Name     string `mapstructure:"name"`
	Server   string `mapstructure:"server"`
	Domain   string `mapstructure:"domain"`
	Interval int    `mapstructure:"interval"`
}

type K8S struct {
	Enabled       bool   `mapstructure:"enabled"`
	Namespace     string `mapstructure:"namespace"`
	LabelSelector string `mapstructure:"label_selector"`
	Interval      int    `mapstructure:"interval"`
}

type ICMP struct {
	Name     string `mapstructure:"name"`
	Host     string `mapstructure:"host"`
	Interval int    `mapstructure:"interval"`
}
