package config

import (
	"fmt"
	"time"
)

type RedisConfig struct {
	Addr       string
	Password   string // empty means no auth
	DB         int    // logical database index
	SessionTTL time.Duration
}

const (
	envVarRedisAddr     = "REDIS_ADDR"
	envVarRedisPassword = "REDIS_PASSWORD"
	envVarRedisDB       = "REDIS_DB"
	envVarRedisTTL      = "SESSION_TTL"
)

const (
	defaultRedisAddr = "localhost:6379"
	defaultRedisDB   = 0
	defaultRedisTTL  = 7 * 24 * time.Hour
)

func LoadRedisConfig() (RedisConfig, error) {
	redisCfg := RedisConfig{
		Addr:       envString(envVarRedisAddr, defaultRedisAddr),
		Password:   envString(envVarRedisPassword, ""),
		DB:         envInt(envVarRedisDB, defaultRedisDB),
		SessionTTL: envDuration(envVarRedisTTL, defaultRedisTTL),
	}

	if err := validateRedisConfig(redisCfg); err != nil {
		return RedisConfig{}, err
	}
	return redisCfg, nil
}

func validateRedisConfig(redisCfg RedisConfig) error {
	if redisCfg.Addr == "" {
		return fmt.Errorf("%s must not be empty", envVarRedisAddr)
	}
	if redisCfg.DB < 0 {
		return fmt.Errorf("%s must not be negative, got %d", envVarRedisDB, redisCfg.DB)
	}
	if redisCfg.SessionTTL <= 0 {
		return fmt.Errorf("%s must be positive, got %s", envVarRedisTTL, redisCfg.SessionTTL)
	}
	return nil
}
