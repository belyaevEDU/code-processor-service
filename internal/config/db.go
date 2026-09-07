package config

import (
	"fmt"
	"net/url"
)

type DBConfig struct {
	URL           string
	MigrationsDir string
}

const (
	envVarDBURL        = "DATABASE_URL"
	envVarDBMigrations = "MIGRATIONS_DIR"
)

const (
	defaultDBURL        = "postgres://postgres:postgres@localhost:5432/remote_code?sslmode=disable"
	defaultDBMigrations = "migrations"
)

func LoadDBConfig() (DBConfig, error) {
	dbCfg := DBConfig{
		URL:           envString(envVarDBURL, defaultDBURL),
		MigrationsDir: envString(envVarDBMigrations, defaultDBMigrations),
	}

	if err := validateDBConfig(dbCfg); err != nil {
		return DBConfig{}, err
	}
	return dbCfg, nil
}

func validateDBConfig(dbCfg DBConfig) error {
	if dbCfg.URL == "" {
		return fmt.Errorf("%s must not be empty", envVarDBURL)
	}
	u, err := url.Parse(dbCfg.URL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return fmt.Errorf("%s must be a valid postgres:// URL, got %q", envVarDBURL, dbCfg.URL)
	}
	if dbCfg.MigrationsDir == "" {
		return fmt.Errorf("%s must not be empty", envVarDBMigrations)
	}
	return nil
}
