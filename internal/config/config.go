// Package config хранит конфигурацию приложения.
// Значения могут быть заданы флагами командной строки (через main.go),
// переменными окружения (SERVERADDRESS, BASEURL, FILE_STORAGE_PATH,
// DATABASE_DSN, SECRET_KEY_FOR_JWT, AUDIT_FILE, AUDIT_URL, ENABLE_HTTPS)
// или файлом конфигурации JSON (путь задаётся флагом -c/-config или CONFIG).
// Приоритет (убывает): переменные окружения → флаги → файл → умолчания.
package config

import (
	"encoding/json"
	"log"
	"os"
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
	// TrustedSubnet — CIDR доверенной подсети для эндпоинта /api/internal/stats.
	// Пустая строка запрещает доступ для всех.
	TrustedSubnet string `env:"TRUSTED_SUBNET"`
}

// FileConfig содержит настройки, загружаемые из JSON-файла конфигурации.
// Все поля — указатели: nil означает «не задано в файле».
type FileConfig struct {
	ServerAddress   *string `json:"server_address"`
	BaseURL         *string `json:"base_url"`
	PathStoreURL    *string `json:"file_storage_path"`
	DatabaseDsn     *string `json:"database_dsn"`
	SecretKeyForJWT *string `json:"secret_key"`
	EnableHTTPS     *bool   `json:"enable_https"`
	AuditFile       *string `json:"audit_file"`
	AuditURL        *string `json:"audit_url"`
	TrustedSubnet   *string `json:"trusted_subnet"`
}

// LoadFileConfig читает и разбирает JSON-файл конфигурации.
// Возвращает nil без ошибки, если path пустой.
func LoadFileConfig(path string) (*FileConfig, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fc FileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	return &fc, nil
}

// InitOptions содержит флаги командной строки и файл конфигурации,
// передаваемые в Init из main. Нулевые значения означают «не задано».
type InitOptions struct {
	ServerAddress   string
	BaseURL         string
	PathStoreURL    string
	DatabaseDsn     string
	SecretKeyForJWT string
	AuditFile       string
	AuditURL        string
	EnableHTTPS     bool
	TrustedSubnet   string
	FileConfig      *FileConfig
}

// AppConfig — глобальный экземпляр конфигурации. Инициализируется вызовом Init.
var AppConfig *Config

// Init инициализирует AppConfig по правилу приоритета:
// переменные окружения → флаги (opts) → файл конфигурации → встроенные умолчания.
func Init(opts InitOptions) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal(err)
	}

	fc := opts.FileConfig

	// strVal выбирает первое непустое значение из цепочки: env → flag → file → default.
	strVal := func(envVal, flagVal string, fileVal *string, def string) string {
		if envVal != "" {
			return envVal
		}
		if flagVal != "" {
			return flagVal
		}
		if fileVal != nil && *fileVal != "" {
			return *fileVal
		}
		return def
	}

	addr := strVal(cfg.ServerAddress, opts.ServerAddress, fcField(fc, func(c *FileConfig) *string { return c.ServerAddress }), "localhost:8080")
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	cfg.ServerAddress = addr

	baseURLVal := strVal(cfg.BaseURL, opts.BaseURL, fcField(fc, func(c *FileConfig) *string { return c.BaseURL }), "http://localhost:8080/")
	if !strings.HasSuffix(baseURLVal, "/") {
		baseURLVal += "/"
	}
	cfg.BaseURL = baseURLVal

	cfg.PathStoreURL = strVal(cfg.PathStoreURL, opts.PathStoreURL, fcField(fc, func(c *FileConfig) *string { return c.PathStoreURL }), "./storage/store_url.txt")
	cfg.DatabaseDsn = strVal(cfg.DatabaseDsn, opts.DatabaseDsn, fcField(fc, func(c *FileConfig) *string { return c.DatabaseDsn }), "")
	cfg.SecretKeyForJWT = strVal(cfg.SecretKeyForJWT, opts.SecretKeyForJWT, fcField(fc, func(c *FileConfig) *string { return c.SecretKeyForJWT }), "")
	cfg.AuditFile = strVal(cfg.AuditFile, opts.AuditFile, fcField(fc, func(c *FileConfig) *string { return c.AuditFile }), "")
	cfg.AuditURL = strVal(cfg.AuditURL, opts.AuditURL, fcField(fc, func(c *FileConfig) *string { return c.AuditURL }), "")
	cfg.TrustedSubnet = strVal(cfg.TrustedSubnet, opts.TrustedSubnet, fcField(fc, func(c *FileConfig) *string { return c.TrustedSubnet }), "")

	if cfg.WorkerNum == 0 {
		cfg.WorkerNum = 5
	}

	// EnableHTTPS: env (already in cfg) → flag → file → false
	if !cfg.EnableHTTPS {
		if opts.EnableHTTPS {
			cfg.EnableHTTPS = true
		} else if fc != nil && fc.EnableHTTPS != nil {
			cfg.EnableHTTPS = *fc.EnableHTTPS
		}
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
	AppConfig = cfg
}

// fcField safely читает поле-указатель из fc, возвращая nil если fc == nil.
func fcField[T any](fc *FileConfig, get func(*FileConfig) *T) *T {
	if fc == nil {
		return nil
	}
	return get(fc)
}
