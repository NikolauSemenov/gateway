package config

import (
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env string `env:"ENV" env-default:"local" env:"ENV"`
	HTTPServer
	Database
	CacheConfig
}

type HTTPServer struct {
	Address     string        `env-default:"localhost:8082" env:"ADDRESS"`
	Timeout     time.Duration `env-default:"5s" env:"TIMEOUT"`
	IdleTimeout time.Duration `env-default:"60s" env:"IDLE_TIMEOUT"`
	AliasLength int           `env-default:"6" env:"ALIAS_LEN"`
}

type Database struct {
	DbType string `env-default:"sqlite" env:"DB_TYPE"`
	DbDsn  string `env:"DB_DSN"`
}

type CacheConfig struct {
	CacheDsn string        `env-default:"redis:6379" env:"CACHE_DSN"`
	CacheTTL time.Duration `env-default:"3600s" env:"CACHE_TTL"`
}

func GetConfig() *Config {
	var cfg Config
	errEnv := cleanenv.ReadEnv(&cfg)
	if errEnv != nil {
		return nil
	}
	return &cfg
}
