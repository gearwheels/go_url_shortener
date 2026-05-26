package main

import (
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/gearwheels/go_url_shortener/internal/audit"
	"github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/gearwheels/go_url_shortener/internal/handler"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	service "github.com/gearwheels/go_url_shortener/internal/service"
	"github.com/gearwheels/go_url_shortener/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Устанавливаются флагами линковщика при сборке:
//
//	go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=2024-01-01 -X main.buildCommit=abc1234"
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func na(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func init() {
	fmt.Printf("Build version: %s\n", na(buildVersion))
	fmt.Printf("Build date: %s\n", na(buildDate))
	fmt.Printf("Build commit: %s\n", na(buildCommit))
}

func main() { // go run "d:\yandex_practice\go_url_shortener\cmd\shortener\main.go" -a localhost:8080 -b http://localhost:8080/
	router := chi.NewRouter()
	router.Use(middleware.RequestID)                 // Добавляет ID каждому запросу
	router.Use(middleware.RealIP)                    // Получает реальный IP
	router.Use(middleware.Recoverer)                 // Обработка паник
	router.Use(middleware.Timeout(60 * time.Second)) // Таймаут

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Кастомизация формата времени
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			return a
		},
	}))

	slog.SetDefault(logger)

	a := flag.String("a", "", "start up address for the server")
	// пробросить в обработчики чтоб отдавать ответ с адресом b
	b := flag.String("b", "", "destination address")
	f := flag.String("f", "", "destination file")
	d := flag.String("d", "", "destination database")
	k := flag.String("k", "", "destination secret")
	s := flag.Bool("s", false, "enable HTTPS (TLS)")
	auditFile := flag.String("audit-file", "", "path to audit log file")
	auditURL := flag.String("audit-url", "", "URL of remote audit receiver")
	var configPath string
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	// разбор командной строки
	flag.Parse()
	// CONFIG env var overrides -c/-config flag
	if envConfig := os.Getenv("CONFIG"); envConfig != "" {
		configPath = envConfig
	}
	fileConfig, err := config.LoadFileConfig(configPath)
	if err != nil {
		slog.Error("Failed to load config file", slog.String("path", configPath), slog.String("err", err.Error()))
	}
	config.Init(*a, *b, *f, *d, *k, *auditFile, *auditURL, *s, fileConfig)

	auditor := audit.NewAuditor()
	if config.AppConfig.AuditFile != "" {
		auditor.Subscribe(audit.NewFileObserver(config.AppConfig.AuditFile))
	}
	if config.AppConfig.AuditURL != "" {
		auditor.Subscribe(audit.NewHTTPObserver(config.AppConfig.AuditURL))
	}
	handler.Auditor = auditor

	tasksDelCh := make(chan schemasshortener.Task, 20)

	// Инициализация сервиса в зависимости от наличия базы данных
	pgExist := config.AppConfig.DatabaseDsn != ""
	var db *sqlx.DB
	if pgExist {
		var err error
		db, err = sqlx.Connect("pgx", config.AppConfig.DatabaseDsn)
		if err != nil {
			slog.Error("Failed to connect to database, using in-memory storage", slog.String("err", err.Error()))
			pgExist = false
		} else {
			defer db.Close()
			if err := runMigrations(db.DB); err != nil {
				slog.Error("Ошибка применения миграций:", "error", err)
			} else {
				slog.Info("Миграции успешно применены. Запуск сервера...")
			}
		}
	}
	service.Shortener = service.GetService(pgExist, db)

	var wg sync.WaitGroup
	wg.Add(1)
	go service.Shortener.WorkerDeleteFromURLTable(tasksDelCh, &wg)

	// Наш middleware для логирования
	router.Use(logrequest.RequestLogger(logger))
	router.Use(logrequest.RequestDataZip())
	router.Use(logrequest.AuthMiddleware)
	router.Post("/", handler.ShortenHandler)
	router.Post("/api/shorten", handler.JSONShortenHandler)
	router.Post("/api/shorten/batch", handler.ShortenBatchHandler)
	router.Get("/{id}", handler.RedirectHandler)
	router.Get("/ping", handler.CheckDBStatus)
	router.Get("/api/user/urls", handler.UserURL)
	router.Delete("/api/user/urls", handler.DeleteBatchHandler(tasksDelCh))

	fmt.Printf("URL Shortener server starting on %s\n", *a)
	fmt.Println("\nEndpoints:")
	fmt.Println("  POST / - Shorten URL")
	fmt.Println("    Content-Type: text/plain")
	fmt.Println("    Body: URL to shorten")
	fmt.Println("    Response: 201 with shortened URL")
	fmt.Println()
	fmt.Println("  GET /{id} - Redirect to original URL")
	fmt.Println("    Response: 307 with Location header")

	if config.AppConfig.EnableHTTPS {
		tlsCfg, err := buildTLSConfig()
		if err != nil {
			slog.Error("Failed to build TLS config", slog.String("err", err.Error()))
			return
		}
		ln, err := tls.Listen("tcp", *a, tlsCfg)
		if err != nil {
			slog.Error("Failed to start HTTPS listener", slog.String("err", err.Error()))
			return
		}
		if err := http.Serve(ln, router); err != nil {
			slog.Error("Server error", slog.String("err", err.Error()))
		}
	} else {
		if err := http.ListenAndServe(*a, router); err != nil {
			slog.Error("Server error", slog.String("err", err.Error()))
		}
	}
}

func runMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("MIGRATION ERROR: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
