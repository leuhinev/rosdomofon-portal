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

func validatePlateFormat(plate string) (bool, string) {
	normalized := normalizePlateNumber(plate)
	plateRegex := regexp.MustCompile(`^[A-Z]\d{3}[A-Z]{2}\d{2,3}$`)

	if !plateRegex.MatchString(normalized) {
		if len(normalized) < 6 {
			return false, "Номер слишком короткий. Пример: A123BC159"
		}
		if len(normalized) > 9 {
			return false, "Номер слишком длинный. Пример: A123BC159"
		}
		hasDigits := regexp.MustCompile(`\d`).MatchString(normalized)
		if !hasDigits {
			return false, "Номер должен содержать цифры. Пример: A123BC159"
		}
		hasLetters := regexp.MustCompile(`[A-Z]`).MatchString(normalized)
		if !hasLetters {
			return false, "Номер должен содержать буквы. Пример: A123BC159"
		}
		return false, "Неверный формат номера. Используйте формат: Буква, 3 цифры, 2 буквы, 2-3 цифры (например: A123BC159)"
	}
	return true, ""
}

func (h *CarsHandler) GetCars(w http.ResponseWriter, r *http.Request) {
	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	cars, err := h.storage.GetCarsByAddressIDs(allowedAddresses)
	if err != nil {
		slog.Error("failed to get cars", "error", err)
		http.Error(w, `{"error":"Ошибка загрузки списка автомобилей"}`, http.StatusInternalServerError)
		return
	}

	// Добавляем информацию об адресе для каждого автомобиля
	type CarWithAddress struct {
		storage.Car
		Address string `json:"address"`
	}

	result := make([]CarWithAddress, len(cars))
	for i, car := range cars {
		result[i] = CarWithAddress{
			Car:     car,
			Address: h.memoryDB.GetAddressByAddressID(car.AddressID),
		}
	}

	json.NewEncoder(w).Encode(result)
}

func (h *CarsHandler) CreateCar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AddressID      int    `json:"address_id"`
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
		http.Error(w, `{"error":"Неверный формат запроса"}`, http.StatusBadRequest)
		return
	}

	if req.AddressID == 0 {
		http.Error(w, `{"error":"Не выбран адрес"}`, http.StatusBadRequest)
		return
	}

	// Проверяем, что address_id принадлежит пользователю
	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)
	addressAllowed := false
	for _, aid := range allowedAddresses {
		if aid == req.AddressID {
			addressAllowed = true
			break
		}
	}
	if !addressAllowed {
		slog.Error("address not accessible", "address_id", req.AddressID)
		http.Error(w, `{"error":"У вас нет доступа к этому адресу"}`, http.StatusForbidden)
		return
	}

	// Получаем строку адреса для валидации
	addressStr := h.memoryDB.GetAddressByAddressID(req.AddressID)
	if addressStr == "" {
		slog.Error("address_id not found", "address_id", req.AddressID)
		http.Error(w, `{"error":"Адрес не найден в системе"}`, http.StatusBadRequest)
		return
	}

	originalPlate := req.PlateNumber
	if originalPlate == "" {
		http.Error(w, `{"error":"Введите номер автомобиля"}`, http.StatusBadRequest)
		return
	}

	isValid, errorMsg := validatePlateFormat(originalPlate)
	if !isValid {
		slog.Error("invalid plate format", "plate", originalPlate, "error", errorMsg)
		http.Error(w, `{"error":"`+errorMsg+`"}`, http.StatusBadRequest)
		return
	}

	normalizedPlate := normalizePlateNumber(originalPlate)

	slog.Info("create car request",
		"address_id", req.AddressID,
		"original_plate", originalPlate,
		"normalized_plate", normalizedPlate,
		"address", addressStr)

	validDays := map[string]bool{"day": true, "week": true, "month": true, "3months": true, "6months": true, "year": true}
	if !validDays[req.ExpiresInDays] {
		slog.Error("invalid expires_in_days", "value", req.ExpiresInDays)
		http.Error(w, `{"error":"Неверно указан срок действия"}`, http.StatusBadRequest)
		return
	}

	exists, err := h.storage.IsCarExists(req.AddressID, normalizedPlate)
	if err != nil {
		slog.Error("failed to check duplicate", "error", err)
		http.Error(w, `{"error":"Ошибка проверки дубликатов"}`, http.StatusInternalServerError)
		return
	}
	if exists {
		slog.Warn("duplicate car", "address_id", req.AddressID, "plate", normalizedPlate)
		http.Error(w, `{"error":"Автомобиль с таким номером уже добавлен для этого адреса"}`, http.StatusConflict)
		return
	}

	days := map[string]int{"day": 1, "week": 7, "month": 30, "3months": 90, "6months": 180, "year": 365}
	daysCount := days[req.ExpiresInDays]

	car := &storage.Car{
		AddressID:      req.AddressID,
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
		http.Error(w, `{"error":"Не удалось добавить автомобиль. Попробуйте позже."}`, http.StatusInternalServerError)
		return
	}

	slog.Info("car created successfully", "car_id", car.ID, "plate", car.PlateNumber)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Автомобиль успешно добавлен",
		"car":     car,
	})
}

func (h *CarsHandler) UpdateCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/cars/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"Неверный идентификатор автомобиля"}`, http.StatusBadRequest)
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
		http.Error(w, `{"error":"Неверный формат запроса"}`, http.StatusBadRequest)
		return
	}

	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	cars, _ := h.storage.GetCarsByAddressIDs(allowedAddresses)
	var addressID int
	var carExists bool
	for _, car := range cars {
		if car.ID == id {
			addressID = car.AddressID
			carExists = true
			break
		}
	}

	if !carExists {
		http.Error(w, `{"error":"Автомобиль не найден"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.UpdateCar(id, addressID, req.Comment, req.AutoOpen, req.NotifyOnDetect, req.NotifyOnEntry, req.NotifyOnExit); err != nil {
		slog.Error("failed to update car", "error", err)
		http.Error(w, `{"error":"Не удалось сохранить изменения"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Изменения сохранены",
	})
}

func (h *CarsHandler) ExtendCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/cars/extend/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"Неверный идентификатор автомобиля"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		AdditionalDays int `json:"additional_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Неверный формат запроса"}`, http.StatusBadRequest)
		return
	}

	if req.AdditionalDays <= 0 {
		http.Error(w, `{"error":"Укажите корректный срок продления"}`, http.StatusBadRequest)
		return
	}

	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	cars, _ := h.storage.GetCarsByAddressIDs(allowedAddresses)
	var addressID int
	var carFound *storage.Car
	for _, car := range cars {
		if car.ID == id {
			addressID = car.AddressID
			carFound = &car
			break
		}
	}

	if addressID == 0 {
		http.Error(w, `{"error":"Автомобиль не найден"}`, http.StatusNotFound)
		return
	}

	daysUntilExpiry := int(time.Until(carFound.ExpiresAt).Hours() / 24)
	if daysUntilExpiry > 7 {
		http.Error(w, `{"error":"Продлить срок можно только за 7 дней до истечения. До истечения осталось `+strconv.Itoa(daysUntilExpiry)+` дней"}`, http.StatusBadRequest)
		return
	}

	if err := h.storage.ExtendCarExpiry(id, addressID, req.AdditionalDays); err != nil {
		slog.Error("failed to extend car", "error", err)
		http.Error(w, `{"error":"Не удалось продлить срок действия"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Срок действия успешно продлён",
	})
}

func (h *CarsHandler) DeleteCar(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/cars/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"Неверный идентификатор автомобиля"}`, http.StatusBadRequest)
		return
	}

	allowedAddresses := r.Context().Value(middleware.AddressIDsKey).([]int)

	cars, _ := h.storage.GetCarsByAddressIDs(allowedAddresses)
	var addressID int
	var carExists bool
	for _, car := range cars {
		if car.ID == id {
			addressID = car.AddressID
			carExists = true
			break
		}
	}

	if !carExists {
		http.Error(w, `{"error":"Автомобиль не найден"}`, http.StatusNotFound)
		return
	}

	if err := h.storage.DeleteCar(id, addressID); err != nil {
		slog.Error("failed to delete car", "error", err)
		http.Error(w, `{"error":"Не удалось удалить автомобиль"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Автомобиль удалён",
	})
}
