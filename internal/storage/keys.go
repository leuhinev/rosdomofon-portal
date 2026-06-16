package storage

import (
	"time"
)

type Key struct {
	ID        int
	AddressID int
	KeyData   string
	Comment   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Storage) CreateKey(key *Key) error {
	dbKey := &UserKey{
		AddressID: key.AddressID,
		KeyData:   key.KeyData,
		Comment:   key.Comment,
	}

	result := s.DB.Create(dbKey)
	if result.Error != nil {
		return result.Error
	}

	key.ID = dbKey.ID
	key.CreatedAt = time.Unix(dbKey.CreatedAt, 0)
	key.UpdatedAt = time.Unix(dbKey.UpdatedAt, 0)
	return nil
}

func (s *Storage) GetKeysByAddressIDs(addressIDs []int) ([]Key, error) {
	if len(addressIDs) == 0 {
		return []Key{}, nil
	}

	var dbKeys []UserKey
	result := s.DB.Where("address_id IN ?", addressIDs).Order("id DESC").Find(&dbKeys)
	if result.Error != nil {
		return nil, result.Error
	}

	keys := make([]Key, len(dbKeys))
	for i, dbKey := range dbKeys {
		keys[i] = Key{
			ID:        dbKey.ID,
			AddressID: dbKey.AddressID,
			KeyData:   dbKey.KeyData,
			Comment:   dbKey.Comment,
			CreatedAt: time.Unix(dbKey.CreatedAt, 0),
			UpdatedAt: time.Unix(dbKey.UpdatedAt, 0),
		}
	}
	return keys, nil
}

func (s *Storage) UpdateKey(id, addressID int, keyData, comment string) error {
	result := s.DB.Model(&UserKey{}).
		Where("id = ? AND address_id = ?", id, addressID).
		Updates(map[string]interface{}{
			"key_data": keyData,
			"comment":  comment,
		})
	return result.Error
}

func (s *Storage) DeleteKey(id, addressID int) error {
	result := s.DB.Where("id = ? AND address_id = ?", id, addressID).Delete(&UserKey{})
	return result.Error
}

func (s *Storage) KeyBelongsToAddress(keyID, addressID int) bool {
	var count int64
	s.DB.Model(&UserKey{}).Where("id = ? AND address_id = ?", keyID, addressID).Count(&count)
	return count > 0
}
