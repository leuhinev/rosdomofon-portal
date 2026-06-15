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
	httpClient  *http.Client
	email       string
	password    string
	serviceID   int
	accessToken string
	tokenExpiry time.Time
	mu          sync.RWMutex
}

func NewClient(email, password string, serviceID int) *Client {
	slog.Debug("creating rosdomofon client", "email", email, "service_id", serviceID)
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		email:      email,
		password:   password,
		serviceID:  serviceID,
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
		"token_preview", tokenResp.AccessToken[:20]+"...")

	return c.accessToken, nil
}

func (c *Client) SendPush(phone int64, code string) error {
	slog.Debug("sending push notification", "phone", phone, "code", code)

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token for push", "error", err)
		return err
	}

	// Форматируем телефон как строку с +7
	phoneStr := fmt.Sprintf("+%d", phone)

	// Отправляем ОДИН объект, а не массив
	request := MessageRequest{
		ToAbonents: []MessageAbonent{
			{Phone: phoneStr},
		},
		Channel:        "notification",
		Message:        fmt.Sprintf("пароль для входа %s", code),
		DeliveryMethod: "push",
	}

	body, err := json.Marshal(request) // Убрали массив, маршалим напрямую объект
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

	// Парсим ответ (может быть объект или массив)
	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err == nil {
		if success, ok := response["success"]; ok && success == false {
			slog.Error("push delivery failed", "result", response["result"])
			return fmt.Errorf("push delivery failed: %v", response["result"])
		}
		if success, ok := response["success"]; ok && success == true {
			slog.Info("push delivered successfully")
		}
	} else {
		// Если ответ - массив
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
	}

	slog.Info("push sent successfully", "phone", phone)
	return nil
}
