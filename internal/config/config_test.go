package config

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func saveEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			env[pair[0]] = pair[1]
		}
	}
	return env
}

func restoreEnv(original map[string]string) {
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			os.Unsetenv(pair[0])
		}
	}
	for k, v := range original {
		os.Setenv(k, v)
	}
}

func unsetAllEnv() {
	vars := []string{
		"RUN_ADDRESS",
		"DATABASE_URI",
		"ACCRUAL_SYSTEM_ADDRESS",
		"JWT_SECRET",
		"CONFIG_PATH",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{os.Args[0]}
}

func TestLoad(t *testing.T) {
	t.Run("загрузка с значениями по умолчанию", func(t *testing.T) {
		origEnv := saveEnv()
		defer restoreEnv(origEnv)

		unsetAllEnv()

		t.Setenv("DATABASE_URI", "postgres://localhost:5432/test")
		t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://localhost:8081")

		resetFlags()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		if cfg.RunAddress != "localhost:8080" {
			t.Errorf("RunAddress = %s, want localhost:8080", cfg.RunAddress)
		}
		if cfg.DatabaseURI != "postgres://localhost:5432/test" {
			t.Errorf("DatabaseURI = %s", cfg.DatabaseURI)
		}
		if cfg.AccrualSystemAddress != "http://localhost:8081" {
			t.Errorf("AccrualSystemAddress = %s", cfg.AccrualSystemAddress)
		}
		if cfg.JWTSecret != "change-me-in-production" {
			t.Errorf("JWTSecret = %s, want change-me-in-production", cfg.JWTSecret)
		}
	})

	t.Run("переменные окружения переопределяют значения по умолчанию", func(t *testing.T) {
		origEnv := saveEnv()
		defer restoreEnv(origEnv)
		unsetAllEnv()

		t.Setenv("RUN_ADDRESS", ":9090")
		t.Setenv("DATABASE_URI", "postgres://prod:5432/proddb")
		t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://accrual:9090")
		t.Setenv("JWT_SECRET", "production-secret")

		resetFlags()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		if cfg.RunAddress != ":9090" {
			t.Errorf("RunAddress = %s, want :9090", cfg.RunAddress)
		}
		if cfg.DatabaseURI != "postgres://prod:5432/proddb" {
			t.Errorf("DatabaseURI = %s", cfg.DatabaseURI)
		}
		if cfg.AccrualSystemAddress != "http://accrual:9090" {
			t.Errorf("AccrualSystemAddress = %s", cfg.AccrualSystemAddress)
		}
		if cfg.JWTSecret != "production-secret" {
			t.Errorf("JWTSecret = %s, want production-secret", cfg.JWTSecret)
		}
	})

	t.Run("отсутствие DATABASE_URI вызывает ошибку", func(t *testing.T) {
		origEnv := saveEnv()
		defer restoreEnv(origEnv)
		unsetAllEnv()

		t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://localhost:8081")

		resetFlags()

		_, err := Load()
		if err == nil {
			t.Error("expected error for missing DATABASE_URI")
		}
	})

	t.Run("отсутствие ACCRUAL_SYSTEM_ADDRESS вызывает ошибку", func(t *testing.T) {
		origEnv := saveEnv()
		defer restoreEnv(origEnv)
		unsetAllEnv()

		t.Setenv("DATABASE_URI", "postgres://localhost:5432/test")

		resetFlags()

		_, err := Load()
		if err == nil {
			t.Error("expected error for missing ACCRUAL_SYSTEM_ADDRESS")
		}
	})

	t.Run("невалидный URL системы начислений", func(t *testing.T) {
		origEnv := saveEnv()
		defer restoreEnv(origEnv)
		unsetAllEnv()

		t.Setenv("DATABASE_URI", "postgres://localhost:5432/test")
		t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "not-a-valid-url://%%%")

		resetFlags()

		_, err := Load()
		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})
}
