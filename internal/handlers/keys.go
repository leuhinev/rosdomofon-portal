package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/middleware"
	"rosdomofon-portal/internal/storage"
	"strconv"
)

type KeysHandler struct {
	storage  *storage.Storage
	memoryDB *memorydb.MemoryDB
}

func NewKeysHandler(s *storage.Storage, mdb *memorydb.MemoryDB) *KeysHandler {
	return &KeysHandler{storage: s, memoryDB: mdb}
}

func (h *KeysHandler) GetKeys(w http.ResponseWriter, r *http.Request) {
	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	keys, err := h.storage.GetKeysByFlatIDs(allowedFlats)
	if err != nil {
		slog.Error("failed to get keys", "error", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(keys)
}

func (h *KeysHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FlatID  int    `json:"flat_id"`
		KeyData string `json:"key_data"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	hexRegex := regexp.MustCompile(`^[0-9A-Fa-f]{1,32}$`)
	if !hexRegex.MatchString(req.KeyData) {
		http.Error(w, `{"error":"invalid key format (hex required)"}`, http.StatusBadRequest)
		return
	}

	if req.FlatID == 0 {
		http.Error(w, `{"error":"flat_id is required"}`, http.StatusBadRequest)
		return
	}

	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)
	flatAllowed := false
	for _, fid := range allowedFlats {
		if fid == req.FlatID {
			flatAllowed = true
			break
		}
	}

	if !flatAllowed {
		http.Error(w, `{"error":"flat not accessible"}`, http.StatusForbidden)
		return
	}

	key := &storage.Key{
		FlatID:  req.FlatID,
		KeyData: req.KeyData,
		Comment: req.Comment,
	}

	if err := h.storage.CreateKey(key); err != nil {
		slog.Error("failed to create key", "error", err)
		http.Error(w, `{"error":"failed to create key"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(key)
}

func (h *KeysHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/keys/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		KeyData string `json:"key_data"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	hexRegex := regexp.MustCompile(`^[0-9A-Fa-f]{1,32}$`)
	if !hexRegex.MatchString(req.KeyData) {
		http.Error(w, `{"error":"invalid key format"}`, http.StatusBadRequest)
		return
	}

	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	keys, _ := h.storage.GetKeysByFlatIDs(allowedFlats)
	var flatID int
	for _, key := range keys {
		if key.ID == id {
			flatID = key.FlatID
			break
		}
	}

	if flatID == 0 {
		http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.UpdateKey(id, flatID, req.KeyData, req.Comment); err != nil {
		slog.Error("failed to update key", "error", err)
		http.Error(w, `{"error":"failed to update"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *KeysHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/keys/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	keys, _ := h.storage.GetKeysByFlatIDs(allowedFlats)
	var flatID int
	for _, key := range keys {
		if key.ID == id {
			flatID = key.FlatID
			break
		}
	}

	if flatID == 0 {
		http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.DeleteKey(id, flatID); err != nil {
		slog.Error("failed to delete key", "error", err)
		http.Error(w, `{"error":"failed to delete"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
