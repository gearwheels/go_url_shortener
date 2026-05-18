package config

import (
	"log"
	"strings"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress   string `env:"SERVERADDRESS"`
	BaseURL         string `env:"BASEURL"`
	PathStoreURL    string `env:"FILE_STORAGE_PATH"`
	DatabaseDsn     string `env:"DATABASE_DSN"`
	SecretKeyForJWT string `env:"SECRET_KEY_FOR_JWT"`
	WorkerNum       int    `env:"WORKER_NUM"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
}

var AppConfig *Config

func Init(serverAddress, baseURL, pathToStoreURL, databaseDsn, secretKeyForJWT, auditFile, auditURL string) {

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal(err)
	}
	if cfg.ServerAddress == "" {
		if strings.HasPrefix(serverAddress, "http://") {
			serverAddress = strings.TrimPrefix(serverAddress, "http://")
		} else if strings.HasPrefix(serverAddress, "https://") {
			serverAddress = strings.TrimPrefix(serverAddress, "https://")
		}
		cfg.ServerAddress = serverAddress
	}
	if cfg.BaseURL == "" {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		cfg.BaseURL = baseURL
	}

	if cfg.PathStoreURL == "" {
		cfg.PathStoreURL = pathToStoreURL
	}
	if cfg.DatabaseDsn == "" {
		cfg.DatabaseDsn = databaseDsn
	}
	if cfg.SecretKeyForJWT == "" {
		cfg.SecretKeyForJWT = secretKeyForJWT
	}
	if cfg.WorkerNum == 0 {
		cfg.WorkerNum = 5
	}
	if cfg.AuditFile == "" {
		cfg.AuditFile = auditFile
	}
	if cfg.AuditURL == "" {
		cfg.AuditURL = auditURL
	}

	AppConfig = &Config{
		ServerAddress:   cfg.ServerAddress,
		BaseURL:         cfg.BaseURL,
		PathStoreURL:    cfg.PathStoreURL,
		DatabaseDsn:     cfg.DatabaseDsn,
		SecretKeyForJWT: cfg.SecretKeyForJWT,
		WorkerNum:       cfg.WorkerNum,
		AuditFile:       cfg.AuditFile,
		AuditURL:        cfg.AuditURL,
	}
}
