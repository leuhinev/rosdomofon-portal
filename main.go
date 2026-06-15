package main

import (
	"context"
	"embed"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"rosdomofon-portal/internal/auth"
	"rosdomofon-portal/internal/config"
	"rosdomofon-portal/internal/handlers"
	"rosdomofon-portal/internal/logger"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/middleware"
	"rosdomofon-portal/internal/rosdomofon"
	"rosdomofon-portal/internal/storage"
)

//go:embed web
var webFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Init(cfg.LogLevel)
	slog.Info("starting portal", "port", cfg.Server.Port)

	mysqlDSN := cfg.MySQL.User + ":" + cfg.MySQL.Password + "@tcp(" + cfg.MySQL.Host + ":" + strconv.Itoa(cfg.MySQL.Port) + ")/" + cfg.MySQL.Database + "?parseTime=true"
	db, err := storage.NewMySQL(mysqlDSN)
	if err != nil {
		slog.Error("failed to connect to mysql", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	memDB := memorydb.New()
	rosClient := rosdomofon.NewClient(cfg.Rosdomofon.Email, cfg.Rosdomofon.Password, cfg.Rosdomofon.ServiceID)
	codeManager := auth.NewCodeManager(cfg.Memcached.Address)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	authHandler := handlers.NewAuthHandler(jwtManager, codeManager, rosClient, memDB)
	flatsHandler := handlers.NewFlatsHandler(memDB)
	carsHandler := handlers.NewCarsHandler(db, memDB)
	keysHandler := handlers.NewKeysHandler(db, memDB)

	mux := http.NewServeMux()

	// Публичные endpoints (без авторизации)
	mux.HandleFunc("POST /api/auth/send-code", authHandler.SendCode)
	mux.HandleFunc("POST /api/auth/verify", authHandler.VerifyCode)

	// Защищенные endpoints (с авторизацией)
	mux.Handle("GET /api/user/flats", middleware.Auth(jwtManager)(http.HandlerFunc(flatsHandler.GetUserFlats)))
	mux.Handle("GET /api/cars", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.GetCars)))
	mux.Handle("POST /api/cars", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.CreateCar)))
	mux.Handle("PUT /api/cars/", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.UpdateCar)))
	mux.Handle("DELETE /api/cars/", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.DeleteCar)))
	mux.Handle("GET /api/keys", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.GetKeys)))
	mux.Handle("POST /api/keys", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.CreateKey)))
	mux.Handle("PUT /api/keys/", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.UpdateKey)))
	mux.Handle("DELETE /api/keys/", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.DeleteKey)))

	// Обработчик для статических файлов и HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Логируем запрос
		slog.Debug("serving file", "path", r.URL.Path)

		// Убираем слеш в начале
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Открываем файл из встроенной FS
		file, err := webFS.Open("web" + path)
		if err != nil {
			slog.Debug("file not found", "path", "web"+path, "error", err)
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		// Получаем информацию о файле
		stat, err := file.Stat()
		if err != nil {
			slog.Error("failed to stat file", "path", path, "error", err)
			http.NotFound(w, r)
			return
		}

		// Устанавливаем правильный Content-Type
		if stat.IsDir() {
			http.NotFound(w, r)
			return
		}

		// Определяем MIME тип по расширению
		contentType := "text/plain"
		if len(path) > 5 && path[len(path)-5:] == ".html" {
			contentType = "text/html; charset=utf-8"
		} else if len(path) > 4 && path[len(path)-4:] == ".css" {
			contentType = "text/css; charset=utf-8"
		} else if len(path) > 3 && path[len(path)-3:] == ".js" {
			contentType = "application/javascript; charset=utf-8"
		}

		w.Header().Set("Content-Type", contentType)

		// Копируем содержимое
		if _, err := io.Copy(w, file); err != nil {
			slog.Error("failed to copy file", "path", path, "error", err)
		}
	})

	handler := middleware.Logging(mux)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запуск синхронизации при старте
	go func() {
		slog.Info("initial sync with Rosdomofon")
		data, err := rosClient.Sync(context.Background())
		if err != nil {
			slog.Error("initial sync failed", "error", err)
		} else {
			memDB.Update(data.PhoneToOwner, data.FlatToAddress, data.OwnerToFlats)
			slog.Info("initial sync completed", "owners", len(data.OwnerToFlats), "flats", len(data.FlatToAddress))
		}
	}()

	// Периодическая синхронизация
	if cfg.Rosdomofon.SyncIntervalMinutes > 0 {
		go func() {
			interval := time.Duration(cfg.Rosdomofon.SyncIntervalMinutes) * time.Minute
			slog.Info("starting periodic sync", "interval_minutes", cfg.Rosdomofon.SyncIntervalMinutes)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				slog.Info("starting periodic sync with Rosdomofon")
				data, err := rosClient.Sync(context.Background())
				if err != nil {
					slog.Error("periodic sync failed", "error", err)
					continue
				}
				memDB.Update(data.PhoneToOwner, data.FlatToAddress, data.OwnerToFlats)
				slog.Info("periodic sync completed", "owners", len(data.OwnerToFlats), "flats", len(data.FlatToAddress))
			}
		}()
	}

	go func() {
		slog.Info("server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("server stopped")
}
