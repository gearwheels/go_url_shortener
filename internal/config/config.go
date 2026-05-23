// Package config хранит конфигурацию приложения.
// Значения могут быть заданы флагами командной строки (через main.go)
// или переменными окружения (SERVERADDRESS, BASEURL, FILE_STORAGE_PATH,
// DATABASE_DSN, SECRET_KEY_FOR_JWT, AUDIT_FILE, AUDIT_URL, ENABLE_HTTPS).
// Переменные окружения имеют приоритет над флагами.
package config

import (
	"log"
	"strings"

	"github.com/caarlos0/env/v6"
)

// Config содержит все настройки сервиса сокращения URL.
type Config struct {
	// ServerAddress — адрес для запуска HTTP-сервера (например "localhost:8080").
	ServerAddress string `env:"SERVERADDRESS"`
	// BaseURL — публичный базовый URL коротких ссылок (например "http://localhost:8080/").
	BaseURL string `env:"BASEURL"`
	// PathStoreURL — путь к файлу-хранилищу для in-memory режима.
	PathStoreURL string `env:"FILE_STORAGE_PATH"`
	// DatabaseDsn — строка подключения к PostgreSQL; пустая строка переключает в in-memory режим.
	DatabaseDsn string `env:"DATABASE_DSN"`
	// SecretKeyForJWT — ключ для подписи HMAC аутентификационных cookie.
	SecretKeyForJWT string `env:"SECRET_KEY_FOR_JWT"`
	// WorkerNum — число воркеров фонового удаления URL (по умолчанию 5).
	WorkerNum int `env:"WORKER_NUM"`
	// AuditFile — путь к файлу аудита; пустая строка отключает FileObserver.
	AuditFile string `env:"AUDIT_FILE"`
	// AuditURL — URL удалённого приёмника событий; пустая строка отключает HTTPObserver.
	AuditURL string `env:"AUDIT_URL"`
	// EnableHTTPS — включает TLS-сервер вместо обычного HTTP.
	EnableHTTPS bool `env:"ENABLE_HTTPS"`
}

// AppConfig — глобальный экземпляр конфигурации. Инициализируется вызовом Init.
var AppConfig *Config

// Init инициализирует AppConfig из переменных окружения и переданных аргументов.
// Переменные окружения имеют приоритет; аргументы используются как значения по умолчанию.
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
		EnableHTTPS:     cfg.EnableHTTPS,
	}
}
