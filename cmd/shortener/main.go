package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gearwheels/go_url_shortener/internal/audit"
	"github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/gearwheels/go_url_shortener/internal/grpcserver"
	"github.com/gearwheels/go_url_shortener/internal/handler"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	service "github.com/gearwheels/go_url_shortener/internal/service"
	"github.com/gearwheels/go_url_shortener/migrations"
	pb "github.com/gearwheels/go_url_shortener/proto"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Устанавливаются флагами линковщика при сборке:
//
//	go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=2024-01-01 -X main.buildCommit=abc1234"
var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

func main() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
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

	a := flag.String("a", "localhost:8080", "start up address for the server")
	b := flag.String("b", "http://localhost:8080/", "destination address")
	f := flag.String("f", "./storage/store_url.txt", "destination file")
	d := flag.String("d", "postgres://shortener:shortener@localhost:5432/shortener", "destination database")
	k := flag.String("k", "", "destination secret")
	g := flag.String("g", "", "gRPC listen address")
	s := flag.Bool("s", false, "enable HTTPS (TLS)")
	t := flag.String("t", "", "trusted subnet CIDR for /api/internal/stats")
	auditFile := flag.String("audit-file", "", "path to audit log file")
	auditURL := flag.String("audit-url", "", "URL of remote audit receiver")
	var configPath string
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	flag.Parse()
	// CONFIG env var overrides -c/-config flag
	if envConfig := os.Getenv("CONFIG"); envConfig != "" {
		configPath = envConfig
	}
	fileConfig, err := config.LoadFileConfig(configPath)
	if err != nil {
		slog.Error("Failed to load config file", slog.String("path", configPath), slog.String("err", err.Error()))
	}
	config.Init(config.InitOptions{
		ServerAddress:   *a,
		BaseURL:         *b,
		PathStoreURL:    *f,
		DatabaseDsn:     *d,
		SecretKeyForJWT: *k,
		AuditFile:       *auditFile,
		AuditURL:        *auditURL,
		EnableHTTPS:     *s,
		TrustedSubnet:   *t,
		GRPCAddress:     *g,
		FileConfig:      fileConfig,
	})

	auditor := audit.NewAuditor()
	if config.AppConfig.AuditFile != "" {
		auditor.Subscribe(audit.NewFileObserver(config.AppConfig.AuditFile))
	}
	if config.AppConfig.AuditURL != "" {
		auditor.Subscribe(audit.NewHTTPObserver(config.AppConfig.AuditURL))
	}
	handler.Auditor = auditor

	tasksDelCh := make(chan schemasshortener.Task, 20)

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
	router.Get("/api/internal/stats", handler.StatsHandler)

	slog.Info("URL Shortener server starting", slog.String("addr", config.AppConfig.ServerAddress))

	srv := &http.Server{
		Addr:    config.AppConfig.ServerAddress,
		Handler: router,
	}

	go func() {
		var serveErr error
		if config.AppConfig.EnableHTTPS {
			tlsCfg, tlsErr := buildTLSConfig()
			if tlsErr != nil {
				slog.Error("Failed to build TLS config", slog.String("err", tlsErr.Error()))
				return
			}
			ln, listenErr := tls.Listen("tcp", config.AppConfig.ServerAddress, tlsCfg)
			if listenErr != nil {
				slog.Error("Failed to start HTTPS listener", slog.String("err", listenErr.Error()))
				return
			}
			serveErr = srv.Serve(ln)
		} else {
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("Server error", slog.String("err", serveErr.Error()))
		}
	}()

	// gRPC server
	grpcOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpcserver.AuthInterceptor),
	}
	if config.AppConfig.EnableHTTPS {
		tlsCfg, tlsErr := buildTLSConfig()
		if tlsErr != nil {
			slog.Error("Failed to build TLS config for gRPC", slog.String("err", tlsErr.Error()))
		} else {
			grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
		}
	}
	grpcSrv := grpc.NewServer(grpcOpts...)
	pb.RegisterShortenerServiceServer(grpcSrv, &grpcserver.ShortenerServer{})

	go func() {
		ln, listenErr := net.Listen("tcp", config.AppConfig.GRPCAddress)
		if listenErr != nil {
			slog.Error("Failed to listen for gRPC", slog.String("addr", config.AppConfig.GRPCAddress), slog.String("err", listenErr.Error()))
			return
		}
		slog.Info("gRPC server starting", slog.String("addr", config.AppConfig.GRPCAddress))
		if serveErr := grpcSrv.Serve(ln); serveErr != nil {
			slog.Error("gRPC server error", slog.String("err", serveErr.Error()))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-quit

	slog.Info("Shutting down server, draining in-flight requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", slog.String("err", err.Error()))
	}

	grpcSrv.GracefulStop()

	close(tasksDelCh)
	wg.Wait()

	slog.Info("Server stopped")
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
