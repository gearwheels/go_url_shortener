package config

import (
	"log"
	"strings"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress string `env:"SERVERADDRESS"` // Адрес сервера
	BaseURL       string `env:"BASEURL"`       // Базовый URL для коротких ссылок
	PathStoreURL  string `env:"FILE_STORAGE_PATH"` 
	DatabaseDsn  string `env:"DATABASE_DSN"` 
}

var AppConfig *Config

func Init(serverAddress string, baseURL string, pathToStoreURL string, databaseDsn string) {

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


	AppConfig = &Config{
		ServerAddress: cfg.ServerAddress,
		BaseURL:       cfg.BaseURL,
		PathStoreURL:  cfg.PathStoreURL,
		DatabaseDsn: cfg.DatabaseDsn,
	}
}
