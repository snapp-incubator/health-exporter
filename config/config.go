package config

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
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
}

type HTTP struct {
	Name          string        `mapstructure:"name"`
	URL           string        `mapstructure:"url"`
	RPS           float64       `mapstructure:"rps"`
	Timeout       time.Duration `mapstructure:"timeout"`
	TLSSkipVerify bool          `mapstructure:"tls_skip_verify"`
}
