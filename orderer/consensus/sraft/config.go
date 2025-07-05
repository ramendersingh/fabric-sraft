package sraft

type Config struct {
	ElectionTimeout   time.Duration
	HeartbeatInterval time.Duration
	MaxBatchSize      int
	Zones             map[string]ZoneConfig
}

type ZoneConfig struct {
	LeaderPriority int
	Nodes          []string
}

func NewConfig(rawConf []byte) (*Config, error) {
	conf := &Config{}
	if err := json.Unmarshal(rawConf, conf); err != nil {
		return nil, err
	}
	return conf, nil
}