package storage

import (
	"time"
)

type Key struct {
	ID        int
	FlatID    int
	KeyData   string
	Comment   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Storage) CreateKey(key *Key) error {
	dbKey := &UserKey{
		FlatID:  key.FlatID,
		KeyData: key.KeyData,
		Comment: key.Comment,
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

func (s *Storage) GetKeysByFlatIDs(flatIDs []int) ([]Key, error) {
	if len(flatIDs) == 0 {
		return []Key{}, nil
	}

	var dbKeys []UserKey
	result := s.DB.Where("flat_id IN ?", flatIDs).Order("id DESC").Find(&dbKeys)
	if result.Error != nil {
		return nil, result.Error
	}

	keys := make([]Key, len(dbKeys))
	for i, dbKey := range dbKeys {
		keys[i] = Key{
			ID:        dbKey.ID,
			FlatID:    dbKey.FlatID,
			KeyData:   dbKey.KeyData,
			Comment:   dbKey.Comment,
			CreatedAt: time.Unix(dbKey.CreatedAt, 0),
			UpdatedAt: time.Unix(dbKey.UpdatedAt, 0),
		}
	}
	return keys, nil
}

func (s *Storage) UpdateKey(id, flatID int, keyData, comment string) error {
	result := s.DB.Model(&UserKey{}).
		Where("id = ? AND flat_id = ?", id, flatID).
		Updates(map[string]interface{}{
			"key_data": keyData,
			"comment":  comment,
		})
	return result.Error
}

func (s *Storage) DeleteKey(id, flatID int) error {
	result := s.DB.Where("id = ? AND flat_id = ?", id, flatID).Delete(&UserKey{})
	return result.Error
}

func (s *Storage) KeyBelongsToFlat(keyID, flatID int) bool {
	var count int64
	s.DB.Model(&UserKey{}).Where("id = ? AND flat_id = ?", keyID, flatID).Count(&count)
	return count > 0
}
