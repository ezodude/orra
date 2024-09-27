package main

import (
	"github.com/vrischmann/envconfig"
)

type Config struct {
	Port       int `envconfig:"default=8005"`
	OpenApiKey string
}

func Load() (Config, error) {
	var cfg Config
	err := envconfig.Init(&cfg)
	if err != nil {
		return Config{}, err
	}
	return cfg, err
}
