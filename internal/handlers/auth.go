package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"rosdomofon-portal/internal/auth"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/rosdomofon"
	"strconv"
)

type AuthHandler struct {
	jwtManager  *auth.JWTManager
	codeManager *auth.CodeManager
	rosClient   *rosdomofon.Client
	memoryDB    *memorydb.MemoryDB
}

func NewAuthHandler(jm *auth.JWTManager, cm *auth.CodeManager, rc *rosdomofon.Client, mdb *memorydb.MemoryDB) *AuthHandler {
	return &AuthHandler{
		jwtManager:  jm,
		codeManager: cm,
		rosClient:   rc,
		memoryDB:    mdb,
	}
}

func (h *AuthHandler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	phoneRegex := regexp.MustCompile(`^(?:\+7|7|8)?(\d{10})$`)
	matches := phoneRegex.FindStringSubmatch(req.Phone)
	if matches == nil {
		slog.Error("invalid phone format", "phone", req.Phone)
		http.Error(w, `{"error":"invalid phone format"}`, http.StatusBadRequest)
		return
	}

	normalizedPhone := "+7" + matches[1]
	slog.Info("normalized phone", "original", req.Phone, "normalized", normalizedPhone)

	_, _, ok := h.memoryDB.GetOwnerByPhone(normalizedPhone)
	if !ok {
		slog.Error("phone not found in Rosdomofon", "phone", normalizedPhone)
		http.Error(w, `{"error":"phone not found in Rosdomofon"}`, http.StatusNotFound)
		return
	}

	if h.codeManager.IsBlocked(normalizedPhone) {
		slog.Warn("phone is blocked", "phone", normalizedPhone)
		http.Error(w, `{"error":"too many attempts, try later"}`, http.StatusTooManyRequests)
		return
	}

	if !h.codeManager.CanSendCode(normalizedPhone) {
		slog.Warn("too frequent requests", "phone", normalizedPhone)
		http.Error(w, `{"error":"wait before requesting new code"}`, http.StatusTooManyRequests)
		return
	}

	code := h.codeManager.GenerateCode()
	slog.Info("generated code", "phone", normalizedPhone, "code", code)

	if err := h.codeManager.SaveCode(normalizedPhone, code); err != nil {
		slog.Error("failed to save code", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	phoneInt, err := strconv.ParseInt(normalizedPhone[1:], 10, 64)
	if err != nil {
		slog.Error("failed to parse phone to int", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("sending push", "phone_int", phoneInt, "code", code)
	if err := h.rosClient.SendPush(phoneInt, code); err != nil {
		slog.Error("failed to send push", "error", err)
		http.Error(w, `{"error":"failed to send push"}`, http.StatusInternalServerError)
		return
	}

	if err := h.codeManager.RecordSend(normalizedPhone); err != nil {
		slog.Error("failed to record send", "error", err)
	}

	slog.Info("code sent successfully", "phone", normalizedPhone)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "code sent"})
}

func (h *AuthHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode verify request", "error", err)
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	phoneRegex := regexp.MustCompile(`^(?:\+7|7|8)?(\d{10})$`)
	matches := phoneRegex.FindStringSubmatch(req.Phone)
	if matches == nil {
		http.Error(w, `{"error":"invalid phone format"}`, http.StatusBadRequest)
		return
	}
	normalizedPhone := "+7" + matches[1]

	if h.codeManager.IsBlocked(normalizedPhone) {
		http.Error(w, `{"error":"too many attempts"}`, http.StatusForbidden)
		return
	}

	valid, err := h.codeManager.VerifyCode(normalizedPhone, req.Code)
	if err != nil || !valid {
		slog.Warn("invalid code", "phone", normalizedPhone, "code", req.Code)
		http.Error(w, `{"error":"invalid code"}`, http.StatusUnauthorized)
		return
	}

	ownerID, addressIDs, ok := h.memoryDB.GetOwnerByPhone(normalizedPhone)
	if !ok {
		slog.Error("user not found after verification", "phone", normalizedPhone)
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	slog.Info("user verified, generating token", "phone", normalizedPhone, "owner_id", ownerID, "address_ids", addressIDs)

	token, err := h.jwtManager.Generate(ownerID, addressIDs)
	if err != nil {
		slog.Error("failed to generate token", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("user verified successfully", "phone", normalizedPhone, "owner_id", ownerID)
	json.NewEncoder(w).Encode(map[string]string{"access_token": token})
}

func (h *AuthHandler) WebViewAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActionToken string `json:"action_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode request", "error", err)
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.ActionToken == "" {
		slog.Error("action_token is empty")
		http.Error(w, `{"error":"action_token required"}`, http.StatusBadRequest)
		return
	}

	slog.Info("WebViewAuth called", "action_token_preview", req.ActionToken[:8]+"...")

	// 1. Проверяем токен в API РосДомофона
	tokenInfo, err := h.rosClient.VerifyActionToken(req.ActionToken)
	if err != nil {
		slog.Error("failed to verify action token", "error", err)
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	subscriberId := tokenInfo.SubscriberId
	slog.Info("action token verified", "subscriber_id", subscriberId)

	// 2. Получаем квартиры абонента из РосДомофона
	abonentFlats, err := h.rosClient.GetAbonentFlats(subscriberId)
	if err != nil {
		slog.Error("failed to get abonent flats", "error", err)
		http.Error(w, `{"error":"failed to get abonent flats"}`, http.StatusInternalServerError)
		return
	}

	if len(abonentFlats) == 0 {
		slog.Warn("no flats found for subscriber", "subscriber_id", subscriberId)
		http.Error(w, `{"error":"no flats found for this subscriber"}`, http.StatusNotFound)
		return
	}

	slog.Info("abonent flats received", "count", len(abonentFlats))

	// 3. Сопоставляем полученные адреса с существующими в нашей БД
	var existingAddressIDs []int

	for _, flat := range abonentFlats {
		addrComp := rosdomofon.AddressComponents{
			StreetID:   flat.Address.Street.ID,
			HouseID:    flat.Address.House.ID,
			EntranceID: flat.Address.Entrance.ID,
			FlatNumber: flat.Address.Flat,
			AddressStr: fmt.Sprintf("%s, %s, д.%s, кв.%d",
				flat.Address.City,
				flat.Address.Street.Name,
				flat.Address.House.Number,
				flat.Address.Flat),
		}

		// Ищем address_id по компонентам в памяти
		addressID, ok := h.memoryDB.GetAddressIDByComponents(addrComp)
		if ok {
			existingAddressIDs = append(existingAddressIDs, addressID)
			slog.Debug("found existing address", "address_id", addressID, "address", addrComp.AddressStr)
		} else {
			slog.Debug("address not found in DB, skipping", "address", addrComp.AddressStr)
		}
	}

	if len(existingAddressIDs) == 0 {
		slog.Warn("no matching addresses found in DB", "subscriber_id", subscriberId)
		http.Error(w, `{"error":"no matching addresses found"}`, http.StatusNotFound)
		return
	}

	// 4. Находим телефон по subscriber_id
	phone, _, ok := h.memoryDB.GetOwnerBySubscriberID(subscriberId)
	if !ok {
		slog.Error("subscriber not found in memory", "subscriber_id", subscriberId)
		http.Error(w, `{"error":"subscriber not found"}`, http.StatusNotFound)
		return
	}

	ownerID, _, _ := h.memoryDB.GetOwnerByPhone(phone)
	slog.Info("subscriber data", "phone", phone, "owner_id", ownerID, "address_ids", existingAddressIDs)

	// 5. Генерируем JWT только с существующими address_id
	jwtToken, err := h.jwtManager.Generate(ownerID, existingAddressIDs)
	if err != nil {
		slog.Error("failed to generate token", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("webview auth successful",
		"subscriber_id", subscriberId,
		"phone", phone,
		"addresses_count", len(existingAddressIDs))

	json.NewEncoder(w).Encode(map[string]string{"access_token": jwtToken})
}
