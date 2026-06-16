package main

import (
	"context"
	"embed"
	"io"
	"io/fs"
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
	rosClient := rosdomofon.NewClient(cfg.Rosdomofon.Email, cfg.Rosdomofon.Password, cfg.Rosdomofon.ServiceTypes)
	codeManager := auth.NewCodeManager(cfg.Memcached.Address)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	authHandler := handlers.NewAuthHandler(jwtManager, codeManager, rosClient, memDB)
	addressesHandler := handlers.NewAddressesHandler(memDB)
	carsHandler := handlers.NewCarsHandler(db, memDB)
	keysHandler := handlers.NewKeysHandler(db, memDB)
	doorsHandler := handlers.NewDoorsHandler(cfg)

	mux := http.NewServeMux()

	// Публичные endpoints
	mux.HandleFunc("POST /api/auth/send-code", authHandler.SendCode)
	mux.HandleFunc("POST /api/auth/verify", authHandler.VerifyCode)
	mux.HandleFunc("POST /api/auth/webview", authHandler.WebViewAuth)

	// Защищенные endpoints (JWT)
	mux.Handle("GET /api/user/addresses", middleware.Auth(jwtManager)(http.HandlerFunc(addressesHandler.GetUserAddresses)))
	mux.Handle("GET /api/cars", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.GetCars)))
	mux.Handle("POST /api/cars", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.CreateCar)))
	mux.Handle("PUT /api/cars/", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.UpdateCar)))
	mux.Handle("POST /api/cars/extend/", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.ExtendCar)))
	mux.Handle("DELETE /api/cars/", middleware.Auth(jwtManager)(http.HandlerFunc(carsHandler.DeleteCar)))
	mux.Handle("GET /api/keys", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.GetKeys)))
	mux.Handle("POST /api/keys", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.CreateKey)))
	mux.Handle("PUT /api/keys/", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.UpdateKey)))
	mux.Handle("DELETE /api/keys/", middleware.Auth(jwtManager)(http.HandlerFunc(keysHandler.DeleteKey)))

	// Старый обработчик для дверей (Basic Auth, без JWT)
	mux.HandleFunc("/door/", doorsHandler.OpenDoorLegacy)

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		slog.Error("failed to load web content", "error", err)
		os.Exit(1)
	}

	staticFS := http.FileServer(http.FS(webContent))
	mux.Handle("/static/", staticFS)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			indexFile, err := webContent.Open("index.html")
			if err != nil {
				slog.Error("failed to open index.html", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			defer indexFile.Close()

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, err := io.Copy(w, indexFile); err != nil {
				slog.Error("failed to copy index.html", "error", err)
			}
			return
		}
		http.NotFound(w, r)
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
			return
		}

		slog.Info("sync data received",
			"unique_phones", len(data.PhoneToOwner),
			"unique_addresses", len(data.AddressToID),
			"unique_owners", len(data.OwnerToAddresses))

		tempToReal := make(map[int]int)
		realAddressToID := make(map[rosdomofon.AddressComponents]int)

		for addrComp, tempID := range data.AddressToID {
			realID, err := db.GetOrCreateAddress(
				addrComp.StreetID,
				addrComp.HouseID,
				addrComp.EntranceID,
				addrComp.FlatNumber,
				addrComp.AddressStr,
			)
			if err != nil {
				slog.Error("failed to save address to DB", "temp_id", tempID, "error", err)
				continue
			}
			tempToReal[tempID] = realID
			realAddressToID[addrComp] = realID
			slog.Debug("address mapped", "temp_id", tempID, "real_id", realID, "address", addrComp.AddressStr)
		}

		newPhoneToOwner := make(map[string]rosdomofon.OwnerInfo)
		for phone, info := range data.PhoneToOwner {
			newInfo := rosdomofon.OwnerInfo{
				OwnerID: info.OwnerID,
			}
			for _, tempID := range info.AddressIDs {
				if realID, ok := tempToReal[tempID]; ok {
					newInfo.AddressIDs = append(newInfo.AddressIDs, realID)
				} else {
					slog.Warn("temp ID not found in real mapping", "temp_id", tempID)
				}
			}
			newPhoneToOwner[phone] = newInfo
		}

		newOwnerToAddresses := make(map[int][]int)
		for ownerID, tempIDs := range data.OwnerToAddresses {
			var realIDs []int
			for _, tempID := range tempIDs {
				if realID, ok := tempToReal[tempID]; ok {
					realIDs = append(realIDs, realID)
				} else {
					slog.Warn("temp ID not found in real mapping for owner", "owner_id", ownerID, "temp_id", tempID)
				}
			}
			if len(realIDs) > 0 {
				newOwnerToAddresses[ownerID] = realIDs
			}
		}

		memDB.Update(newPhoneToOwner, realAddressToID, newOwnerToAddresses)

		slog.Info("initial sync completed",
			"owners", len(newOwnerToAddresses),
			"addresses", len(realAddressToID))
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

				tempToReal := make(map[int]int)
				realAddressToID := make(map[rosdomofon.AddressComponents]int)

				for addrComp, tempID := range data.AddressToID {
					realID, err := db.GetOrCreateAddress(
						addrComp.StreetID,
						addrComp.HouseID,
						addrComp.EntranceID,
						addrComp.FlatNumber,
						addrComp.AddressStr,
					)
					if err != nil {
						slog.Error("failed to save address to DB", "temp_id", tempID, "error", err)
						continue
					}
					tempToReal[tempID] = realID
					realAddressToID[addrComp] = realID
				}

				newPhoneToOwner := make(map[string]rosdomofon.OwnerInfo)
				for phone, info := range data.PhoneToOwner {
					newInfo := rosdomofon.OwnerInfo{
						OwnerID: info.OwnerID,
					}
					for _, tempID := range info.AddressIDs {
						if realID, ok := tempToReal[tempID]; ok {
							newInfo.AddressIDs = append(newInfo.AddressIDs, realID)
						}
					}
					newPhoneToOwner[phone] = newInfo
				}

				newOwnerToAddresses := make(map[int][]int)
				for ownerID, tempIDs := range data.OwnerToAddresses {
					var realIDs []int
					for _, tempID := range tempIDs {
						if realID, ok := tempToReal[tempID]; ok {
							realIDs = append(realIDs, realID)
						}
					}
					if len(realIDs) > 0 {
						newOwnerToAddresses[ownerID] = realIDs
					}
				}

				memDB.Update(newPhoneToOwner, realAddressToID, newOwnerToAddresses)
				slog.Info("periodic sync completed", "owners", len(newOwnerToAddresses), "addresses", len(realAddressToID))
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
