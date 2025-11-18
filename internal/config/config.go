package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/miekg/dns"
	"gopkg.in/yaml.v3"
)

const (
	defaultListenAddr  = ":9876"
	defaultHTTPTimeout = 3 * time.Second
	defaultDNSTimeout  = 2 * time.Second
	defaultICMPTimeout = 2 * time.Second
	defaultK8sRPS      = 1.0
)

type Config struct {
	Listen  string  `yaml:"listen"`
	Targets Targets `yaml:"targets"`
}

type Targets struct {
	HTTP []HTTPTarget `yaml:"http"`
	DNS  []DNSTarget  `yaml:"dns"`
	K8S  K8STarget    `yaml:"k8s"`
	ICMP []ICMPTarget `yaml:"icmp"`
}

type HTTPTarget struct {
	Name              string        `yaml:"name"`
	URL               string        `yaml:"url"`
	RPS               float64       `yaml:"rps"`
	Timeout           time.Duration `yaml:"timeout"`
	TLSSkipVerify     bool          `yaml:"tls_skip_verify"`
	DisableKeepAlives bool          `yaml:"disable_keepalives"`
	H2cEnabled        bool          `yaml:"h2c_enabled"`
	Host              string        `yaml:"host"`
}

type DNSTarget struct {
	Name       string        `yaml:"name"`
	Domain     string        `yaml:"domain"`
	RecordType string        `yaml:"record_type"`
	RPS        float64       `yaml:"rps"`
	ServerIP   string        `yaml:"server_ip"`
	ServerPort int           `yaml:"server_port"`
	Timeout    time.Duration `yaml:"timeout"`
}

type K8STarget struct {
	Enabled     bool             `yaml:"enabled"`
	SimpleProbe []K8SSimpleProbe `yaml:"simple-probe"`
}

type K8SSimpleProbe struct {
	NameSpace string  `yaml:"namespace"`
	RPS       float64 `yaml:"rps"`
}

type ICMPTarget struct {
	Name    string        `yaml:"name"`
	Host    string        `yaml:"host"`
	TTL     int           `yaml:"ttl"`
	RPS     float64       `yaml:"rps"`
	Timeout time.Duration `yaml:"timeout"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.setDefaults(); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) setDefaults() error {
	if c.Listen == "" {
		c.Listen = defaultListenAddr
	}

	for i := range c.Targets.HTTP {
		if c.Targets.HTTP[i].Timeout <= 0 {
			c.Targets.HTTP[i].Timeout = defaultHTTPTimeout
		}
	}

	defaultServer, err := lookupDefaultDNSServer()
	if err != nil {
		return err
	}

	for i := range c.Targets.DNS {
		if c.Targets.DNS[i].Timeout <= 0 {
			c.Targets.DNS[i].Timeout = defaultDNSTimeout
		}
		if c.Targets.DNS[i].RecordType == "" {
			c.Targets.DNS[i].RecordType = "A"
		}
		if c.Targets.DNS[i].ServerIP == "" {
			c.Targets.DNS[i].ServerIP = defaultServer
		}
		if c.Targets.DNS[i].ServerPort == 0 {
			c.Targets.DNS[i].ServerPort = 53
		}
	}

	for i := range c.Targets.K8S.SimpleProbe {
		if c.Targets.K8S.SimpleProbe[i].RPS <= 0 {
			c.Targets.K8S.SimpleProbe[i].RPS = defaultK8sRPS
		}
	}

	for i := range c.Targets.ICMP {
		if c.Targets.ICMP[i].Timeout <= 0 {
			c.Targets.ICMP[i].Timeout = defaultICMPTimeout
		}
		if c.Targets.ICMP[i].TTL <= 0 {
			c.Targets.ICMP[i].TTL = 64
		}
	}

	return nil
}

func (c Config) validate() error {
	if len(c.Targets.HTTP) == 0 &&
		len(c.Targets.DNS) == 0 &&
		len(c.Targets.ICMP) == 0 &&
		(!c.Targets.K8S.Enabled || len(c.Targets.K8S.SimpleProbe) == 0) {
		return errors.New("no probes configured")
	}

	for _, h := range c.Targets.HTTP {
		if h.Name == "" {
			return errors.New("http target name is required")
		}
		if h.URL == "" {
			return fmt.Errorf("http target %q: url is required", h.Name)
		}
		if h.RPS <= 0 {
			return fmt.Errorf("http target %q: rps should be > 0", h.Name)
		}
	}

	for _, d := range c.Targets.DNS {
		if d.Name == "" {
			return errors.New("dns target name is required")
		}
		if d.Domain == "" {
			return fmt.Errorf("dns target %q: domain is required", d.Name)
		}
		if d.RPS <= 0 {
			return fmt.Errorf("dns target %q: rps should be > 0", d.Name)
		}
	}

	if c.Targets.K8S.Enabled {
		if len(c.Targets.K8S.SimpleProbe) == 0 {
			return errors.New("k8s probes enabled but no namespace configured")
		}
		for _, sp := range c.Targets.K8S.SimpleProbe {
			if sp.NameSpace == "" {
				return errors.New("k8s simple probe namespace is required")
			}
		}
	}

	for _, icmp := range c.Targets.ICMP {
		if icmp.Name == "" {
			return errors.New("icmp target name is required")
		}
		if icmp.Host == "" {
			return fmt.Errorf("icmp target %q: host is required", icmp.Name)
		}
		if icmp.RPS <= 0 {
			return fmt.Errorf("icmp target %q: rps should be > 0", icmp.Name)
		}
	}

	return nil
}

func lookupDefaultDNSServer() (string, error) {
	cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return "", fmt.Errorf("load resolv.conf: %w", err)
	}
	if len(cfg.Servers) == 0 {
		return "", errors.New("no dns servers found in resolv.conf")
	}
	return cfg.Servers[0], nil
}
