package rosdomofon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	httpClient   *http.Client
	email        string
	password     string
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
	serviceTypes []string
	serviceInfo  map[int]ServiceInfo
}

func NewClient(email, password string, serviceTypes []string) *Client {
	slog.Info("creating rosdomofon client", "email", email, "service_types", serviceTypes)
	return &Client{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		email:        email,
		password:     password,
		serviceTypes: serviceTypes,
		serviceInfo:  make(map[int]ServiceInfo),
	}
}

func (c *Client) GetToken() (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.mu.RUnlock()
		slog.Debug("using cached token", "expires_at", c.tokenExpiry)
		return token, nil
	}
	c.mu.RUnlock()

	slog.Debug("token expired or not exists, refreshing")
	return c.refreshToken()
}

func (c *Client) refreshToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data := fmt.Sprintf("grant_type=password&client_id=machine&username=%s&password=%s", c.email, c.password)
	slog.Debug("requesting token from rosdomofon", "url", "https://rdba.rosdomofon.com/authserver-service/oauth/token")

	req, err := http.NewRequest("POST", "https://rdba.rosdomofon.com/authserver-service/oauth/token", bytes.NewBufferString(data))
	if err != nil {
		slog.Error("failed to create token request", "error", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send token request", "error", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read token response", "error", err)
		return "", err
	}

	slog.Debug("token response status", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		slog.Error("token request failed", "status", resp.StatusCode, "body", string(body))
		return "", fmt.Errorf("token request failed: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		slog.Error("failed to parse token response", "error", err, "body", string(body))
		return "", err
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	slog.Info("token obtained successfully",
		"expires_in", tokenResp.ExpiresIn,
		"expires_at", c.tokenExpiry,
		"token_preview", c.accessToken[:20]+"...")

	return c.accessToken, nil
}

// VerifyActionToken - проверка токена из WebView
func (c *Client) VerifyActionToken(actionToken string) (*ActionTokenInfo, error) {
	slog.Info("verifying action token", "token_preview", actionToken[:8]+"...")

	machineToken, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get machine token", "error", err)
		return nil, err
	}

	url := fmt.Sprintf("https://rdba.rosdomofon.com/abonents-service/api/v1/action_token/verify/%s", actionToken)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("failed to create verify request", "error", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+machineToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send verify request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read verify response", "error", err)
		return nil, err
	}

	slog.Info("action token verify response", "status", resp.StatusCode, "body", string(body))

	if resp.StatusCode != http.StatusOK {
		slog.Error("action token verification failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("action token verification failed: %d", resp.StatusCode)
	}

	var info ActionTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		slog.Error("failed to parse verify response", "error", err)
		return nil, err
	}

	slog.Info("action token verified", "subscriber_id", info.SubscriberId)
	return &info, nil
}

// GetAbonentFlats - получение всех квартир абонента
func (c *Client) GetAbonentFlats(subscriberId int) ([]AbonentFlat, error) {
	slog.Info("getting abonent flats", "subscriber_id", subscriberId)

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token", "error", err)
		return nil, err
	}

	url := fmt.Sprintf("https://rdba.rosdomofon.com/abonents-service/api/v1/abonents/%d/flats", subscriberId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("failed to create abonent flats request", "error", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send abonent flats request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read abonent flats response", "error", err)
		return nil, err
	}

	slog.Info("abonent flats response", "status", resp.StatusCode, "body_length", len(body))

	if resp.StatusCode != http.StatusOK {
		slog.Error("abonent flats request failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("abonent flats request failed: %d", resp.StatusCode)
	}

	var flats []AbonentFlat
	if err := json.Unmarshal(body, &flats); err != nil {
		slog.Error("failed to parse abonent flats response", "error", err)
		return nil, err
	}

	slog.Info("abonent flats loaded", "count", len(flats))
	return flats, nil
}

// GetConnections - получает все связи для конкретного сервиса
func (c *Client) GetConnections(serviceID int) ([]Connection, error) {
	slog.Info("getting connections for service", "service_id", serviceID)

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token", "error", err)
		return nil, err
	}

	url := fmt.Sprintf("https://rdba.rosdomofon.com/abonents-service/api/v1/services/%d/connections", serviceID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("failed to create connections request", "error", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send connections request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read connections response", "error", err)
		return nil, err
	}

	slog.Info("connections response", "status", resp.StatusCode, "body_length", len(body))

	if resp.StatusCode != http.StatusOK {
		slog.Error("connections request failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("connections request failed: %d", resp.StatusCode)
	}

	var connections []Connection
	if err := json.Unmarshal(body, &connections); err != nil {
		slog.Error("failed to parse connections response", "error", err)
		return nil, err
	}

	slog.Info("connections loaded", "count", len(connections))
	return connections, nil
}

// getEntrancesWithServices - получает список подъездов с услугами и сохраняет serviceInfo
func (c *Client) getEntrancesWithServices() ([]EntranceItem, error) {
	slog.Info("getting entrances with services")

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token", "error", err)
		return nil, err
	}

	url := "https://rdba.rosdomofon.com/abonents-service/api/v1/entrances"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("failed to create entrances request", "error", err)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send entrances request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read entrances response", "error", err)
		return nil, err
	}

	slog.Info("entrances response", "status", resp.StatusCode, "body_length", len(body))

	if resp.StatusCode != http.StatusOK {
		slog.Error("entrances request failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("entrances request failed: %d", resp.StatusCode)
	}

	var response EntranceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		slog.Error("failed to parse entrances response", "error", err)
		return nil, err
	}

	c.mu.Lock()
	for _, item := range response.Content {
		for _, svc := range item.Services {
			c.serviceInfo[svc.ID] = ServiceInfo{
				ID:   svc.ID,
				Type: svc.Type,
			}
		}
	}
	c.mu.Unlock()

	slog.Info("entrances loaded", "count", len(response.Content), "services_count", len(c.serviceInfo))
	return response.Content, nil
}

// GetServiceInfo - возвращает информацию о сервисе по ID
func (c *Client) GetServiceInfo(serviceID int) (ServiceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.serviceInfo[serviceID]
	return info, ok
}

// GetFilteredServiceIDs - возвращает список service_id, удовлетворяющих фильтру по типам
func (c *Client) GetFilteredServiceIDs() ([]int, error) {
	slog.Info("getting filtered service IDs", "types", c.serviceTypes)

	entrances, err := c.getEntrancesWithServices()
	if err != nil {
		return nil, err
	}

	serviceIDs := make(map[int]bool)
	serviceTypesSet := make(map[string]bool)
	for _, st := range c.serviceTypes {
		serviceTypesSet[st] = true
	}

	if len(c.serviceTypes) == 0 {
		slog.Warn("no service types in config, loading all services")
	}

	for _, entrance := range entrances {
		for _, svc := range entrance.Services {
			shouldInclude := len(c.serviceTypes) == 0 || serviceTypesSet[svc.Type]
			if shouldInclude {
				serviceIDs[svc.ID] = true
				slog.Debug("service accepted", "id", svc.ID, "type", svc.Type)
			} else {
				slog.Debug("service filtered out", "id", svc.ID, "type", svc.Type)
			}
		}
	}

	var ids []int
	for id := range serviceIDs {
		ids = append(ids, id)
	}

	slog.Info("filtered services", "total", len(ids), "types", c.serviceTypes)
	return ids, nil
}

// SendPush - отправка кода подтверждения (существующий метод)
func (c *Client) SendPush(phone int64, code string) error {
	slog.Debug("sending push notification", "phone", phone, "code", code)

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token for push", "error", err)
		return err
	}

	phoneStr := fmt.Sprintf("+%d", phone)

	request := MessageRequest{
		ToAbonents: []MessageAbonent{
			{Phone: phoneStr},
		},
		Channel:        "notification",
		Message:        fmt.Sprintf("пароль для входа %s", code),
		DeliveryMethod: "push",
	}

	body, err := json.Marshal(request)
	if err != nil {
		slog.Error("failed to marshal push request", "error", err)
		return err
	}

	slog.Info("push request body", "body", string(body))

	req, err := http.NewRequest("POST", "https://rdba.rosdomofon.com/abonents-service/api/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		slog.Error("failed to create push request", "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send push request", "error", err)
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read push response", "error", err)
		return err
	}

	slog.Info("push response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode != http.StatusOK {
		slog.Error("push send failed", "status", resp.StatusCode)
		return fmt.Errorf("push send failed: %d", resp.StatusCode)
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(respBody, &responses); err == nil {
		for i, r := range responses {
			if success, ok := r["success"]; ok && success == false {
				slog.Error("push delivery failed", "index", i, "result", r["result"])
				return fmt.Errorf("push delivery failed: %v", r["result"])
			}
			if success, ok := r["success"]; ok && success == true {
				slog.Info("push delivered successfully", "index", i)
			}
		}
	}

	slog.Info("push sent successfully", "phone", phone)
	return nil
}

// SendNotification - отправка произвольного уведомления через push
func (c *Client) SendNotification(phone int64, message string) error {
	slog.Debug("sending notification", "phone", phone, "message", message)

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token for notification", "error", err)
		return err
	}

	phoneStr := fmt.Sprintf("+%d", phone)

	request := MessageRequest{
		ToAbonents: []MessageAbonent{
			{Phone: phoneStr},
		},
		Channel:        "notification",
		Message:        message,
		DeliveryMethod: "push",
	}

	body, err := json.Marshal(request)
	if err != nil {
		slog.Error("failed to marshal notification request", "error", err)
		return err
	}

	slog.Info("notification request body", "body", string(body))

	req, err := http.NewRequest("POST", "https://rdba.rosdomofon.com/abonents-service/api/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		slog.Error("failed to create notification request", "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send notification request", "error", err)
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read notification response", "error", err)
		return err
	}

	slog.Info("notification response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode != http.StatusOK {
		slog.Error("notification send failed", "status", resp.StatusCode)
		return fmt.Errorf("notification send failed: %d", resp.StatusCode)
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(respBody, &responses); err == nil {
		for i, r := range responses {
			if success, ok := r["success"]; ok && success == false {
				slog.Error("notification delivery failed", "index", i, "result", r["result"])
				return fmt.Errorf("notification delivery failed: %v", r["result"])
			}
			if success, ok := r["success"]; ok && success == true {
				slog.Info("notification delivered successfully", "index", i)
			}
		}
	}

	slog.Info("notification sent successfully", "phone", phone)
	return nil
}
