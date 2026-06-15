package rosdomofon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type OwnerInfo struct {
	OwnerID int
	FlatIDs []int
}

type SyncedData struct {
	PhoneToOwner  map[string]OwnerInfo
	FlatToAddress map[int]string
	OwnerToFlats  map[int][]int
}

func (c *Client) Sync(ctx context.Context) (*SyncedData, error) {
	slog.Info("starting sync with Rosdomofon API")

	token, err := c.GetToken()
	if err != nil {
		slog.Error("failed to get token for sync", "error", err)
		return nil, fmt.Errorf("get token: %w", err)
	}
	slog.Debug("token obtained for sync", "token_preview", token[:20]+"...")

	slog.Info("requesting entrances", "service_id", c.serviceID)

	entrances, err := c.getEntrances(token)
	if err != nil {
		slog.Error("failed to get entrances", "error", err)
		return nil, fmt.Errorf("get entrances: %w", err)
	}

	slog.Info("got entrances response", "count", len(entrances))

	// Логируем первые 3 подъезда для отладки
	for i, entrance := range entrances {
		if i < 3 {
			slog.Debug("entrance details",
				"index", i,
				"entrance_id", entrance.Entrance.ID,
				"entrance_number", entrance.Entrance.Number,
				"house_id", entrance.House.ID,
				"house_number", entrance.House.Number,
				"street_id", entrance.Street.ID,
				"street_name", entrance.Street.Name,
				"city", entrance.City)
		}
	}

	data := &SyncedData{
		PhoneToOwner:  make(map[string]OwnerInfo),
		FlatToAddress: make(map[int]string),
		OwnerToFlats:  make(map[int][]int),
	}

	for i, entrance := range entrances {
		entranceID := entrance.Entrance.ID
		slog.Info("processing entrance",
			"progress", fmt.Sprintf("%d/%d", i+1, len(entrances)),
			"entrance_id", entranceID,
			"entrance_number", entrance.Entrance.Number,
			"house_number", entrance.House.Number)

		flats, err := c.getFlats(token, entranceID)
		if err != nil {
			slog.Error("failed to get flats for entrance",
				"entrance_id", entranceID,
				"error", err)
			continue
		}

		slog.Info("got flats for entrance",
			"entrance_id", entranceID,
			"flats_count", len(flats))

		for flatIdx, flat := range flats {
			phoneStr := fmt.Sprintf("+%d", flat.Owner.Phone)
			address := fmt.Sprintf("%s, %s, д.%s, кв.%d",
				flat.Address.City,
				flat.Address.Street.Name,
				flat.Address.House.Number,
				flat.Address.Flat)

			// Логируем каждую квартиру в DEBUG режиме
			if flatIdx < 5 {
				slog.Debug("flat details",
					"flat_id", flat.ID,
					"owner_id", flat.Owner.ID,
					"owner_phone", phoneStr,
					"address", address,
					"entrance_id", entranceID,
					"virtual", flat.Virtual)
			}

			data.FlatToAddress[flat.ID] = address

			info := data.PhoneToOwner[phoneStr]
			info.OwnerID = flat.Owner.ID
			info.FlatIDs = append(info.FlatIDs, flat.ID)
			data.PhoneToOwner[phoneStr] = info

			data.OwnerToFlats[flat.Owner.ID] = append(data.OwnerToFlats[flat.Owner.ID], flat.ID)
		}
	}

	slog.Info("sync statistics",
		"unique_phones", len(data.PhoneToOwner),
		"total_flats", len(data.FlatToAddress),
		"unique_owners", len(data.OwnerToFlats))

	if len(data.PhoneToOwner) == 0 {
		slog.Warn("NO OWNERS FOUND! Check if there are any flats with owners in the response")
	} else {
		slog.Info("owners found", "count", len(data.PhoneToOwner))
		for phone, info := range data.PhoneToOwner {
			slog.Info("owner data", "phone", phone, "owner_id", info.OwnerID, "flats", info.FlatIDs)
		}
	}

	return data, nil
}

func (c *Client) getEntrances(token string) ([]Entrance, error) {
	url := fmt.Sprintf("https://rdba.rosdomofon.com/abonents-service/api/v1/services/%d/entrances", c.serviceID)
	slog.Info("making API request to get entrances",
		"url", url,
		"service_id", c.serviceID)

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

	slog.Info("entrances API response",
		"status_code", resp.StatusCode,
		"response_size_bytes", len(body))

	if resp.StatusCode != http.StatusOK {
		slog.Error("entrances request failed with non-200 status",
			"status", resp.StatusCode,
			"response_preview", string(body[:min(len(body), 500)]))
		return nil, fmt.Errorf("entrances request failed: %d", resp.StatusCode)
	}

	var entrances []Entrance
	if err := json.Unmarshal(body, &entrances); err != nil {
		slog.Error("failed to parse entrances JSON",
			"error", err)
		return nil, err
	}

	// Получаем первый ID подъезда для статистики
	firstID := 0
	if len(entrances) > 0 {
		firstID = entrances[0].Entrance.ID
	}

	slog.Info("entrances parsed successfully",
		"count", len(entrances),
		"first_entrance_id", firstID)

	return entrances, nil
}

func (c *Client) getFlats(token string, entranceID int) ([]Flat, error) {
	url := fmt.Sprintf("https://rdba.rosdomofon.com/abonents-service/api/v1/entrances/%d/flats", entranceID)
	slog.Info("making API request to get flats",
		"url", url,
		"entrance_id", entranceID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("failed to create flats request", "error", err, "entrance_id", entranceID)
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send flats request", "error", err, "entrance_id", entranceID)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read flats response", "error", err, "entrance_id", entranceID)
		return nil, err
	}

	slog.Info("flats API response",
		"entrance_id", entranceID,
		"status_code", resp.StatusCode,
		"response_size_bytes", len(body))

	if resp.StatusCode != http.StatusOK {
		slog.Error("flats request failed with non-200 status",
			"entrance_id", entranceID,
			"status", resp.StatusCode,
			"response_preview", string(body[:min(len(body), 500)]))
		return nil, fmt.Errorf("flats request failed: %d", resp.StatusCode)
	}

	var flats []Flat
	if err := json.Unmarshal(body, &flats); err != nil {
		slog.Error("failed to parse flats JSON",
			"error", err,
			"entrance_id", entranceID)
		return nil, err
	}

	slog.Info("flats parsed successfully",
		"entrance_id", entranceID,
		"count", len(flats))

	if len(flats) > 0 {
		slog.Debug("first flat sample",
			"entrance_id", entranceID,
			"flat_id", flats[0].ID,
			"owner_id", flats[0].Owner.ID,
			"owner_phone", flats[0].Owner.Phone)
	}

	return flats, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
