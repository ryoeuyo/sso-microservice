// Package config — загрузка конфига из YAML + переопределение env-переменными
// (env приоритетнее). Путь к конфигу берётся из флага --config или CONFIG_PATH.
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env    string `yaml:"env"    env:"SSO_ENV"    env-default:"local"`
	GRPC   GRPC   `yaml:"grpc"`
	DB     DB     `yaml:"db"`
	Tokens Tokens `yaml:"tokens"`
}

type GRPC struct {
	Port    int           `yaml:"port"    env:"SSO_GRPC_PORT"    env-default:"44044"`
	Timeout time.Duration `yaml:"timeout" env:"SSO_GRPC_TIMEOUT" env-default:"10s"`
}

type DB struct {
	DSN string `yaml:"dsn" env:"SSO_DB_DSN" env-required:"true"`
}

type Tokens struct {
	AccessTTL  time.Duration `yaml:"access_ttl"  env:"SSO_ACCESS_TTL"  env-default:"15m"`
	RefreshTTL time.Duration `yaml:"refresh_ttl" env:"SSO_REFRESH_TTL" env-default:"720h"`
}

// MustLoad — для main: при ошибке падаем с понятным сообщением и кодом 1.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %s\n", err)
		os.Exit(1)
	}
	return cfg
}

// Load — путь к YAML берётся (в порядке убывания приоритета):
//   1. флаг --config;
//   2. env CONFIG_PATH;
//   3. ./config/local.yaml.
//
// Если файла нет — это не ошибка, конфиг можно собрать только из env
// (тогда SSO_DB_DSN обязателен).
func Load() (*Config, error) {
	path := configPath()

	cfg := &Config{}
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := cleanenv.ReadConfig(path, cfg); err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			return cfg, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
	}

	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read env: %w", err)
	}
	return cfg, nil
}

func configPath() string {
	var path string
	fs := flag.NewFlagSet("sso", flag.ContinueOnError)
	fs.StringVar(&path, "config", "", "path to YAML config")
	_ = fs.Parse(os.Args[1:])

	if path != "" {
		return path
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		return env
	}
	return "./config/local.yaml"
}
