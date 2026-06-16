package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"rosdomofon-portal/internal/config"
	"rosdomofon-portal/internal/doors"
	"strings"
)

type DoorsHandler struct {
	doorsConfig map[string]config.Door
	authUsers   map[string]string // login -> password
}

func NewDoorsHandler(cfg *config.Config) *DoorsHandler {
	return &DoorsHandler{
		doorsConfig: cfg.Doors,
		authUsers:   cfg.Auth,
	}
}

// OpenDoorLegacy обрабатывает запросы по старому пути /door/{id}/open с Basic Auth
func (h *DoorsHandler) OpenDoorLegacy(w http.ResponseWriter, r *http.Request) {
	// 1. Проверка Basic Auth
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	expectedPass, exists := h.authUsers[username]
	if !exists || expectedPass != password {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 2. Парсинг URL: /door/1/open -> извлекаем номер двери 1
	path := strings.TrimPrefix(r.URL.Path, "/door/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "open" {
		http.Error(w, "Invalid request format. Use /door/{id}/open", http.StatusBadRequest)
		return
	}
	doorID := parts[0]

	// 3. Поиск конфигурации двери
	doorCfg, exists := h.doorsConfig[doorID]
	if !exists {
		http.Error(w, fmt.Sprintf("Door %s not found", doorID), http.StatusNotFound)
		return
	}

	// 4. Проксирование запроса
	d := doors.Door{
		URL:         doorCfg.URL,
		Method:      doorCfg.Method,
		AuthType:    doorCfg.AuthType,
		Username:    doorCfg.Username,
		Password:    doorCfg.Password,
		Body:        doorCfg.Body,
		ContentType: doorCfg.ContentType,
		InsecureTLS: doorCfg.InsecureTLS,
	}

	slog.Info("Opening door (legacy)", "door_id", doorID, "url", d.URL)

	resp, err := doors.ProxyRequest(d)
	if err != nil {
		slog.Error("Failed to proxy door request", "door_id", doorID, "error", err)
		http.Error(w, fmt.Sprintf("Device error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	slog.Info("Door opened (legacy)", "door_id", doorID, "status", resp.StatusCode)
}
