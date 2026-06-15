package memorydb

import (
	"rosdomofon-portal/internal/rosdomofon"
	"sync"
)

type MemoryDB struct {
	mu            sync.RWMutex
	phoneToOwner  map[string]rosdomofon.OwnerInfo
	flatToAddress map[int]string
	ownerToFlats  map[int][]int
	ownerToPhone  map[int]string // Обратный индекс: owner_id -> phone
}

func New() *MemoryDB {
	return &MemoryDB{
		phoneToOwner:  make(map[string]rosdomofon.OwnerInfo),
		flatToAddress: make(map[int]string),
		ownerToFlats:  make(map[int][]int),
		ownerToPhone:  make(map[int]string),
	}
}

func (db *MemoryDB) Update(data map[string]rosdomofon.OwnerInfo, flats map[int]string, ownerFlats map[int][]int) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.phoneToOwner = data
	db.flatToAddress = flats
	db.ownerToFlats = ownerFlats

	// Строим обратный индекс owner_id -> phone
	db.ownerToPhone = make(map[int]string)
	for phone, info := range data {
		db.ownerToPhone[info.OwnerID] = phone
	}
}

func (db *MemoryDB) GetOwnerByPhone(phone string) (int, []int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	info, ok := db.phoneToOwner[phone]
	if !ok {
		return 0, nil, false
	}
	return info.OwnerID, info.FlatIDs, true
}

func (db *MemoryDB) GetPhoneByOwnerID(ownerID int) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	phone, ok := db.ownerToPhone[ownerID]
	return phone, ok
}

func (db *MemoryDB) GetFlatsByOwner(ownerID int) []int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.ownerToFlats[ownerID]
}

func (db *MemoryDB) GetAddress(flatID int) string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.flatToAddress[flatID]
}

func (db *MemoryDB) FlatBelongsToOwner(ownerID, flatID int) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	flats, ok := db.ownerToFlats[ownerID]
	if !ok {
		return false
	}
	for _, f := range flats {
		if f == flatID {
			return true
		}
	}
	return false
}
