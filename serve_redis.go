package allino

import (
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	URL        string `json:"url"`
	ClusterURL string `json:"cluster_url"`
}

func (c *RedisConfig) connect() (redis.UniversalClient, error) {
	if c.URL != "" {
		opt, err := redis.ParseURL(c.URL)
		if err != nil {
			return nil, err
		}

		return redis.NewClient(opt), nil
	} else if c.ClusterURL != "" {
		opt, err := redis.ParseClusterURL(c.ClusterURL)
		if err != nil {
			return nil, err
		}

		return redis.NewClusterClient(opt), nil
	}

	return nil, nil
}
