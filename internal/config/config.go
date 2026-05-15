package config

import (
	"bufio"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	JWTSecret            string
}

func Load() (*Config, error) {
	cfg := &Config{
		RunAddress:           "localhost:8080",
		JWTSecret:            "change-me-in-production",
		AccrualSystemAddress: "http://localhost:8081",
	}

	loadFromEnvFile(cfg)

	loadFromFlags(cfg)

	loadFromEnv(cfg)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadFromEnvFile(cfg *Config) {
	envFiles := []string{".env", "configs/.env"}

	for _, filePath := range envFiles {
		if err := parseEnvFile(filePath, cfg); err == nil {
			return
		}
	}
}

func parseEnvFile(filePath string, cfg *Config) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		applyEnvValue(cfg, key, value)
	}

	return scanner.Err()
}

func loadFromFlags(cfg *Config) {
	flagRunAddress := flag.String("a", cfg.RunAddress, "HTTP server address")
	flagDatabaseURI := flag.String("d", cfg.DatabaseURI, "PostgreSQL connection URI")
	flagAccrualAddr := flag.String("r", cfg.AccrualSystemAddress, "Accrual system base URL")

	if !flag.Parsed() {
		flag.Parse()
	}

	if isFlagSet("a") {
		cfg.RunAddress = *flagRunAddress
	}
	if isFlagSet("d") {
		cfg.DatabaseURI = *flagDatabaseURI
	}
	if isFlagSet("r") {
		cfg.AccrualSystemAddress = *flagAccrualAddr
	}
}

func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func loadFromEnv(cfg *Config) {
	cfg.RunAddress = cmp.Or(os.Getenv("RUN_ADDRESS"), cfg.RunAddress)
	cfg.DatabaseURI = cmp.Or(os.Getenv("DATABASE_URI"), cfg.DatabaseURI)
	cfg.AccrualSystemAddress = cmp.Or(os.Getenv("ACCRUAL_SYSTEM_ADDRESS"), cfg.AccrualSystemAddress)
	cfg.JWTSecret = cmp.Or(os.Getenv("JWT_SECRET"), cfg.JWTSecret)
}

func applyEnvValue(cfg *Config, key, value string) {
	if value == "" {
		return
	}

	switch key {
	case "RUN_ADDRESS":
		cfg.RunAddress = value
	case "DATABASE_URI":
		cfg.DatabaseURI = value
	case "ACCRUAL_SYSTEM_ADDRESS":
		cfg.AccrualSystemAddress = value
	case "JWT_SECRET":
		cfg.JWTSecret = value
	}
}

func (c *Config) validate() error {
	if c.DatabaseURI == "" {
		return errors.New("database URI is required")
	}
	if c.AccrualSystemAddress == "" {
		return errors.New("accrual system address is required")
	}
	if _, err := url.Parse(c.AccrualSystemAddress); err != nil {
		return fmt.Errorf("invalid accrual system address: %w", err)
	}
	return nil
}

func (c *Config) MaskedDatabaseURI() string {
	if c.DatabaseURI == "" {
		return ""
	}

	u, err := url.Parse(c.DatabaseURI)
	if err != nil {
		return "invalid-uri"
	}

	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}

	return u.String()
}
