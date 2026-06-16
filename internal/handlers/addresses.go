package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/middleware"
)

type AddressesHandler struct {
	memoryDB *memorydb.MemoryDB
}

func NewAddressesHandler(mdb *memorydb.MemoryDB) *AddressesHandler {
	return &AddressesHandler{memoryDB: mdb}
}

func (h *AddressesHandler) GetUserAddresses(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(middleware.OwnerIDKey).(int)
	if !ok {
		slog.Error("owner_id not found in context")
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	slog.Info("GetUserAddresses called", "owner_id", ownerID)

	addressIDs := h.memoryDB.GetAddressesByOwner(ownerID)
	slog.Info("addresses found", "owner_id", ownerID, "count", len(addressIDs), "ids", addressIDs)

	addresses := make([]map[string]interface{}, 0, len(addressIDs))
	for _, addressID := range addressIDs {
		addresses = append(addresses, map[string]interface{}{
			"address_id": addressID,
			"address":    h.memoryDB.GetAddressByAddressID(addressID),
		})
	}

	json.NewEncoder(w).Encode(addresses)
}
