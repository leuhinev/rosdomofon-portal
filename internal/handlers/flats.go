package handlers

import (
	"encoding/json"
	"net/http"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/middleware"
)

type FlatsHandler struct {
	memoryDB *memorydb.MemoryDB
}

func NewFlatsHandler(mdb *memorydb.MemoryDB) *FlatsHandler {
	return &FlatsHandler{memoryDB: mdb}
}

func (h *FlatsHandler) GetUserFlats(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(middleware.OwnerIDKey).(int)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	flatIDs := h.memoryDB.GetFlatsByOwner(ownerID)
	flats := make([]map[string]interface{}, 0, len(flatIDs))
	for _, flatID := range flatIDs {
		flats = append(flats, map[string]interface{}{
			"flat_id": flatID,
			"address": h.memoryDB.GetAddress(flatID),
		})
	}

	json.NewEncoder(w).Encode(flats)
}
