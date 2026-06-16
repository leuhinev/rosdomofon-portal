package rosdomofon

import (
	"context"
	"fmt"
	"log/slog"
)

type AddressComponents struct {
	StreetID   int
	HouseID    int
	EntranceID int
	FlatNumber int
	AddressStr string
}

type SyncedData struct {
	PhoneToOwner     map[string]OwnerInfo
	AddressToID      map[AddressComponents]int // map[компоненты адреса] -> address_id
	OwnerToAddresses map[int][]int             // owner_id -> []address_id
}

type OwnerInfo struct {
	OwnerID    int
	AddressIDs []int // теперь храним address_id вместо flat_id
}

func (c *Client) Sync(ctx context.Context) (*SyncedData, error) {
	slog.Info("starting sync with Rosdomofon API")

	serviceIDs, err := c.GetFilteredServiceIDs()
	if err != nil {
		slog.Error("failed to get filtered service IDs", "error", err)
		return nil, fmt.Errorf("get filtered services: %w", err)
	}

	slog.Info("filtered service IDs", "count", len(serviceIDs), "ids", serviceIDs)

	if len(serviceIDs) == 0 {
		slog.Warn("no services matched the filter")
		return &SyncedData{
			PhoneToOwner:     make(map[string]OwnerInfo),
			AddressToID:      make(map[AddressComponents]int),
			OwnerToAddresses: make(map[int][]int),
		}, nil
	}

	data := &SyncedData{
		PhoneToOwner:     make(map[string]OwnerInfo),
		AddressToID:      make(map[AddressComponents]int),
		OwnerToAddresses: make(map[int][]int),
	}

	// Временный кеш для address_id, чтобы не создавать дубли
	addressCache := make(map[AddressComponents]int)

	for _, serviceID := range serviceIDs {
		svcInfo, ok := c.GetServiceInfo(serviceID)
		svcType := ""
		if ok {
			svcType = svcInfo.Type
		}
		slog.Info("processing service", "service_id", serviceID, "type", svcType)

		connections, err := c.GetConnections(serviceID)
		if err != nil {
			slog.Error("failed to get connections for service", "service_id", serviceID, "error", err)
			continue
		}

		slog.Info("processing connections for service", "service_id", serviceID, "count", len(connections))

		for _, conn := range connections {
			if conn.Blocked || conn.Account.Blocked {
				slog.Debug("skipping blocked connection", "connection_id", conn.ID)
				continue
			}

			flat := conn.Flat
			addressStr := fmt.Sprintf("%s, %s, д.%s, кв.%d",
				flat.Address.City,
				flat.Address.Street.Name,
				flat.Address.House.Number,
				flat.Address.Flat)

			// Создаём компоненты адреса
			addrComp := AddressComponents{
				StreetID:   flat.Address.Street.ID,
				HouseID:    flat.Address.House.ID,
				EntranceID: flat.Address.Entrance.ID,
				FlatNumber: flat.Address.Flat,
				AddressStr: addressStr,
			}

			// Получаем или создаём address_id
			addressID, ok := addressCache[addrComp]
			if !ok {
				// В реальном коде здесь нужно создавать запись в БД через storage
				// Но в sync мы только собираем данные, БД обновляется отдельно
				// Поэтому просто генерируем временный ID для этого цикла
				addressID = len(addressCache) + 1
				addressCache[addrComp] = addressID
				data.AddressToID[addrComp] = addressID
			}

			// Используем владельца из account
			phone := fmt.Sprintf("+%d", conn.Account.Owner.Phone)
			ownerID := conn.Account.Owner.ID

			// Обновляем данные по владельцу
			info := data.PhoneToOwner[phone]
			info.OwnerID = ownerID
			// Добавляем address_id, если его ещё нет
			found := false
			for _, aid := range info.AddressIDs {
				if aid == addressID {
					found = true
					break
				}
			}
			if !found {
				info.AddressIDs = append(info.AddressIDs, addressID)
			}
			data.PhoneToOwner[phone] = info

			// Добавляем в ownerToAddresses
			addresses := data.OwnerToAddresses[ownerID]
			foundAddr := false
			for _, aid := range addresses {
				if aid == addressID {
					foundAddr = true
					break
				}
			}
			if !foundAddr {
				data.OwnerToAddresses[ownerID] = append(data.OwnerToAddresses[ownerID], addressID)
			}

			slog.Debug("processed connection",
				"address_id", addressID,
				"address", addressStr,
				"owner_id", ownerID,
				"phone", phone)
		}
	}

	slog.Info("sync completed",
		"unique_phones", len(data.PhoneToOwner),
		"unique_addresses", len(data.AddressToID),
		"unique_owners", len(data.OwnerToAddresses))

	return data, nil
}
