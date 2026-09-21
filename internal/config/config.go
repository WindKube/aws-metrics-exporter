package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

const EnvPrefix = "AWSME"

type Config struct {
	Global    Global    `mapstructure:"global"    json:"global"`
	Storage   Storage   `mapstructure:"storage"   json:"storage"`
	Resources []string  `mapstructure:"resources" json:"resources"`
	Accounts  []Account `mapstructure:"accounts"  json:"accounts"`
}

type Global struct {
	ScrapeInterval time.Duration `mapstructure:"scrape_interval" json:"scrape_interval"`
	Regions        []string      `mapstructure:"regions"         json:"regions"`
	Log            Log           `mapstructure:"log"             json:"log"`
	Health         Health        `mapstructure:"health"          json:"health"`
	Temporal       Temporal      `mapstructure:"temporal"        json:"temporal"`
}

type Log struct {
	Level  string `mapstructure:"level"  json:"level"`
	Format string `mapstructure:"format" json:"format"`
}

type Health struct {
	Address       string        `mapstructure:"address"        json:"address"`
	ProbeInterval time.Duration `mapstructure:"probe_interval" json:"probe_interval"`
}

type Temporal struct {
	Address   string      `mapstructure:"address"    json:"address"`
	Namespace string      `mapstructure:"namespace"  json:"namespace"`
	TaskQueue string      `mapstructure:"task_queue" json:"task_queue"`
	TLS       TemporalTLS `mapstructure:"tls"        json:"tls"`
}

type TemporalTLS struct {
	Enabled            bool   `mapstructure:"enabled"              json:"enabled"`
	CertFile           string `mapstructure:"cert_file"            json:"cert_file"`
	KeyFile            string `mapstructure:"key_file"             json:"key_file"`
	CAFile             string `mapstructure:"ca_file"              json:"ca_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify" json:"insecure_skip_verify"`
}

type Storage struct {
	Backend       string        `mapstructure:"backend"       json:"backend"`
	Elasticsearch Elasticsearch `mapstructure:"elasticsearch" json:"elasticsearch"`
}

type Elasticsearch struct {
	Addresses   []string `mapstructure:"addresses"    json:"addresses"`
	Username    string   `mapstructure:"username"     json:"-"`
	Password    string   `mapstructure:"password"     json:"-"`
	APIKey      string   `mapstructure:"api_key"      json:"-"`
	IndexPrefix string   `mapstructure:"index_prefix" json:"index_prefix"`
	BatchSize   int      `mapstructure:"batch_size"   json:"batch_size"`
}

// Account is embedded in Temporal Schedule action arguments, so its fields are part of the
// workflow payload contract.
type Account struct {
	Name           string        `mapstructure:"name"            json:"name"`
	ID             string        `mapstructure:"id"              json:"id"`
	RoleARN        string        `mapstructure:"role_arn"        json:"role_arn"`
	ExternalID     string        `mapstructure:"external_id"     json:"external_id,omitempty"`
	Regions        []string      `mapstructure:"regions"         json:"regions"`
	Resources      []string      `mapstructure:"resources"       json:"resources"`
	ScrapeInterval time.Duration `mapstructure:"scrape_interval" json:"scrape_interval"`
}

func (c *Config) Account(name string) (Account, bool) {
	for _, a := range c.Accounts {
		if a.Name == name {
			return a, true
		}
	}
	return Account{}, false
}

// Load reads the YAML file at path, applies defaults and AWSME_* environment overrides, and
// resolves every account's effective regions, resources and scrape interval.
func Load(path string, knownResources []string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("global.scrape_interval", time.Hour)
	v.SetDefault("global.log.level", "info")
	v.SetDefault("global.log.format", "json")
	v.SetDefault("global.health.address", ":8080")
	v.SetDefault("global.health.probe_interval", 15*time.Second)
	v.SetDefault("global.temporal.address", "localhost:7233")
	v.SetDefault("global.temporal.namespace", "default")
	v.SetDefault("global.temporal.task_queue", "aws-metrics-exporter")
	v.SetDefault("storage.backend", "elasticsearch")
	v.SetDefault("storage.elasticsearch.index_prefix", "aws-inventory")
	v.SetDefault("storage.elasticsearch.batch_size", 500)

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// AutomaticEnv alone is invisible to Unmarshal for keys absent from the file, and credentials
	// are deliberately absent from the file.
	for _, key := range []string{
		"storage.elasticsearch.username",
		"storage.elasticsearch.password",
		"storage.elasticsearch.api_key",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, err
		}
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	cfg.resolve()

	if err := cfg.Validate(knownResources); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) resolve() {
	for i := range c.Accounts {
		a := &c.Accounts[i]
		if len(a.Regions) == 0 {
			a.Regions = c.Global.Regions
		}
		if len(a.Resources) == 0 {
			a.Resources = c.Resources
		}
		if a.ScrapeInterval == 0 {
			a.ScrapeInterval = c.Global.ScrapeInterval
		}
	}
}
