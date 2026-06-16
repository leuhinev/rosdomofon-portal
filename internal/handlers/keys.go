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
	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	keys, err := h.storage.GetKeysByAddressIDs(allowedAddresses)
	if err != nil {
		slog.Error("failed to get keys", "error", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	// Добавляем информацию об адресе для каждого ключа
	type KeyWithAddress struct {
		storage.Key
		Address string `json:"address"`
	}

	result := make([]KeyWithAddress, len(keys))
	for i, key := range keys {
		result[i] = KeyWithAddress{
			Key:     key,
			Address: h.memoryDB.GetAddressByAddressID(key.AddressID),
		}
	}

	json.NewEncoder(w).Encode(result)
}

func (h *KeysHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AddressID int    `json:"address_id"`
		KeyData   string `json:"key_data"`
		Comment   string `json:"comment"`
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

	if req.AddressID == 0 {
		http.Error(w, `{"error":"address_id is required"}`, http.StatusBadRequest)
		return
	}

	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)
	addressAllowed := false
	for _, aid := range allowedAddresses {
		if aid == req.AddressID {
			addressAllowed = true
			break
		}
	}
	if !addressAllowed {
		http.Error(w, `{"error":"address not accessible"}`, http.StatusForbidden)
		return
	}

	key := &storage.Key{
		AddressID: req.AddressID,
		KeyData:   req.KeyData,
		Comment:   req.Comment,
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

	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	keys, _ := h.storage.GetKeysByAddressIDs(allowedAddresses)
	var addressID int
	for _, key := range keys {
		if key.ID == id {
			addressID = key.AddressID
			break
		}
	}

	if addressID == 0 {
		http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.UpdateKey(id, addressID, req.KeyData, req.Comment); err != nil {
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

	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	keys, _ := h.storage.GetKeysByAddressIDs(allowedAddresses)
	var addressID int
	for _, key := range keys {
		if key.ID == id {
			addressID = key.AddressID
			break
		}
	}

	if addressID == 0 {
		http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.DeleteKey(id, addressID); err != nil {
		slog.Error("failed to delete key", "error", err)
		http.Error(w, `{"error":"failed to delete"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
