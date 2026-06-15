package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/middleware"
	"rosdomofon-portal/internal/storage"
	"strconv"
	"strings"
	"time"
)

type CarsHandler struct {
	storage  *storage.Storage
	memoryDB *memorydb.MemoryDB
}

func NewCarsHandler(s *storage.Storage, mdb *memorydb.MemoryDB) *CarsHandler {
	return &CarsHandler{storage: s, memoryDB: mdb}
}

func normalizePlateNumber(plate string) string {
	translit := map[rune]rune{
		'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H',
		'О': 'O', 'Р': 'P', 'С': 'C', 'Т': 'T', 'У': 'Y', 'Х': 'X',
		'а': 'a', 'в': 'b', 'е': 'e', 'к': 'k', 'м': 'm', 'н': 'h',
		'о': 'o', 'р': 'p', 'с': 'c', 'т': 't', 'у': 'y', 'х': 'x',
	}

	result := strings.Map(func(r rune) rune {
		if val, ok := translit[r]; ok {
			return val
		}
		return r
	}, plate)

	return strings.ToUpper(result)
}

func (h *CarsHandler) GetCars(w http.ResponseWriter, r *http.Request) {
	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	cars, err := h.storage.GetCarsByFlatIDs(allowedFlats)
	if err != nil {
		slog.Error("failed to get cars", "error", err)
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(cars)
}

func (h *CarsHandler) CreateCar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FlatID         int    `json:"flat_id"`
		PlateNumber    string `json:"plate_number"`
		Comment        string `json:"comment"`
		AutoOpen       bool   `json:"auto_open"`
		NotifyOnDetect bool   `json:"notify_on_detect"`
		NotifyOnEntry  bool   `json:"notify_on_entry"`
		NotifyOnExit   bool   `json:"notify_on_exit"`
		ExpiresInDays  string `json:"expires_in_days"`
	}

	body, _ := io.ReadAll(r.Body)
	slog.Info("raw request body", "body", string(body))
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	originalPlate := req.PlateNumber
	normalizedPlate := normalizePlateNumber(req.PlateNumber)

	slog.Info("create car request",
		"flat_id", req.FlatID,
		"original_plate", originalPlate,
		"normalized_plate", normalizedPlate)

	plateRegex := regexp.MustCompile(`^[A-Z]\d{3}[A-Z]{2}\d{2,3}$`)
	if !plateRegex.MatchString(normalizedPlate) {
		slog.Error("invalid plate format", "plate", normalizedPlate)
		http.Error(w, `{"error":"invalid plate format"}`, http.StatusBadRequest)
		return
	}

	if req.FlatID == 0 {
		slog.Error("flat_id is required")
		http.Error(w, `{"error":"flat_id is required"}`, http.StatusBadRequest)
		return
	}

	validDays := map[string]bool{"day": true, "week": true, "month": true, "3months": true, "6months": true, "year": true}
	if !validDays[req.ExpiresInDays] {
		slog.Error("invalid expires_in_days", "value", req.ExpiresInDays)
		http.Error(w, `{"error":"invalid expires_in_days"}`, http.StatusBadRequest)
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
		slog.Error("flat not accessible", "flat_id", req.FlatID)
		http.Error(w, `{"error":"flat not accessible"}`, http.StatusForbidden)
		return
	}

	days := map[string]int{"day": 1, "week": 7, "month": 30, "3months": 90, "6months": 180, "year": 365}
	daysCount := days[req.ExpiresInDays]

	car := &storage.Car{
		FlatID:         req.FlatID,
		PlateNumber:    normalizedPlate,
		Comment:        req.Comment,
		AutoOpen:       req.AutoOpen,
		NotifyOnDetect: req.NotifyOnDetect,
		NotifyOnEntry:  req.NotifyOnEntry,
		NotifyOnExit:   req.NotifyOnExit,
		ExpiresAt:      time.Now().AddDate(0, 0, daysCount),
	}

	if err := h.storage.CreateCar(car); err != nil {
		slog.Error("failed to create car", "error", err)
		http.Error(w, `{"error":"failed to create car"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("car created successfully", "car_id", car.ID, "plate", car.PlateNumber)
	json.NewEncoder(w).Encode(car)
}

func (h *CarsHandler) UpdateCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/cars/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Comment        string `json:"comment"`
		AutoOpen       bool   `json:"auto_open"`
		NotifyOnDetect bool   `json:"notify_on_detect"`
		NotifyOnEntry  bool   `json:"notify_on_entry"`
		NotifyOnExit   bool   `json:"notify_on_exit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	cars, _ := h.storage.GetCarsByFlatIDs(allowedFlats)
	var flatID int
	for _, car := range cars {
		if car.ID == id {
			flatID = car.FlatID
			break
		}
	}

	if flatID == 0 {
		http.Error(w, `{"error":"car not found"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.UpdateCar(id, flatID, req.Comment, req.AutoOpen, req.NotifyOnDetect, req.NotifyOnEntry, req.NotifyOnExit); err != nil {
		slog.Error("failed to update car", "error", err)
		http.Error(w, `{"error":"failed to update"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *CarsHandler) ExtendCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/cars/extend/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		AdditionalDays int `json:"additional_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	cars, _ := h.storage.GetCarsByFlatIDs(allowedFlats)
	var flatID int
	var carFound *storage.Car
	for _, car := range cars {
		if car.ID == id {
			flatID = car.FlatID
			carFound = &car
			break
		}
	}

	if flatID == 0 {
		http.Error(w, `{"error":"car not found"}`, http.StatusNotFound)
		return
	}

	daysUntilExpiry := int(time.Until(carFound.ExpiresAt).Hours() / 24)
	if daysUntilExpiry > 7 {
		http.Error(w, `{"error":"can only extend within 7 days of expiry"}`, http.StatusBadRequest)
		return
	}

	if err := h.storage.ExtendCarExpiry(id, flatID, req.AdditionalDays); err != nil {
		slog.Error("failed to extend car", "error", err)
		http.Error(w, `{"error":"failed to extend"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "extended"})
}

func (h *CarsHandler) DeleteCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/cars/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	allowedFlats := r.Context().Value(middleware.FlatIDsKey).([]int)

	cars, _ := h.storage.GetCarsByFlatIDs(allowedFlats)
	var flatID int
	for _, car := range cars {
		if car.ID == id {
			flatID = car.FlatID
			break
		}
	}

	if flatID == 0 {
		http.Error(w, `{"error":"car not found"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.DeleteCar(id, flatID); err != nil {
		slog.Error("failed to delete car", "error", err)
		http.Error(w, `{"error":"failed to delete"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
