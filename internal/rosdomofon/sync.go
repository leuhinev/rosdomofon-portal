package rosdomofon

import (
	"context"
	"fmt"
	"log/slog"
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

	serviceIDs, err := c.GetFilteredServiceIDs()
	if err != nil {
		slog.Error("failed to get filtered service IDs", "error", err)
		return nil, fmt.Errorf("get filtered services: %w", err)
	}

	slog.Info("filtered service IDs", "count", len(serviceIDs), "ids", serviceIDs)

	if len(serviceIDs) == 0 {
		slog.Warn("no services matched the filter")
		return &SyncedData{
			PhoneToOwner:  make(map[string]OwnerInfo),
			FlatToAddress: make(map[int]string),
			OwnerToFlats:  make(map[int][]int),
		}, nil
	}

	data := &SyncedData{
		PhoneToOwner:  make(map[string]OwnerInfo),
		FlatToAddress: make(map[int]string),
		OwnerToFlats:  make(map[int][]int),
	}

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
			address := fmt.Sprintf("%s, %s, д.%s, кв.%d",
				flat.Address.City,
				flat.Address.Street.Name,
				flat.Address.House.Number,
				flat.Address.Flat)

			data.FlatToAddress[flat.ID] = address

			// Используем владельца из account, а не из flat
			phone := fmt.Sprintf("+%d", conn.Account.Owner.Phone)
			ownerID := conn.Account.Owner.ID

			info := data.PhoneToOwner[phone]
			info.OwnerID = ownerID
			found := false
			for _, fid := range info.FlatIDs {
				if fid == flat.ID {
					found = true
					break
				}
			}
			if !found {
				info.FlatIDs = append(info.FlatIDs, flat.ID)
			}
			data.PhoneToOwner[phone] = info

			flats := data.OwnerToFlats[ownerID]
			foundFlat := false
			for _, fid := range flats {
				if fid == flat.ID {
					foundFlat = true
					break
				}
			}
			if !foundFlat {
				data.OwnerToFlats[ownerID] = append(data.OwnerToFlats[ownerID], flat.ID)
			}

			slog.Debug("processed connection",
				"flat_id", flat.ID,
				"owner_id", ownerID,
				"phone", phone,
				"address", address)
		}
	}

	slog.Info("sync completed",
		"unique_phones", len(data.PhoneToOwner),
		"total_flats", len(data.FlatToAddress),
		"unique_owners", len(data.OwnerToFlats))

	return data, nil
}
