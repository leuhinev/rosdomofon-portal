package memorydb

import (
	"log/slog"
	"rosdomofon-portal/internal/rosdomofon"
	"sync"
)

type MemoryDB struct {
	mu                 sync.RWMutex
	phoneToOwner       map[string]rosdomofon.OwnerInfo
	addressIDToAddress map[int]string                       // address_id -> строка адреса
	addressToID        map[rosdomofon.AddressComponents]int // компоненты -> address_id
	ownerToAddresses   map[int][]int                        // owner_id -> []address_id
	ownerToPhone       map[int]string
	subscriberToPhone  map[int]string
}

func New() *MemoryDB {
	return &MemoryDB{
		phoneToOwner:       make(map[string]rosdomofon.OwnerInfo),
		addressIDToAddress: make(map[int]string),
		addressToID:        make(map[rosdomofon.AddressComponents]int),
		ownerToAddresses:   make(map[int][]int),
		ownerToPhone:       make(map[int]string),
		subscriberToPhone:  make(map[int]string),
	}
}

func (db *MemoryDB) Update(
	phoneToOwner map[string]rosdomofon.OwnerInfo,
	addressToID map[rosdomofon.AddressComponents]int,
	ownerToAddresses map[int][]int,
) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.phoneToOwner = phoneToOwner
	db.addressToID = addressToID
	db.ownerToAddresses = ownerToAddresses

	// Строим обратный индекс address_id -> address_str
	db.addressIDToAddress = make(map[int]string)
	for addrComp, id := range addressToID {
		db.addressIDToAddress[id] = addrComp.AddressStr
	}

	// Строим обратные индексы для телефонов
	db.ownerToPhone = make(map[int]string)
	db.subscriberToPhone = make(map[int]string)
	for phone, info := range phoneToOwner {
		db.ownerToPhone[info.OwnerID] = phone
		db.subscriberToPhone[info.OwnerID] = phone
	}

	slog.Debug("memorydb updated",
		"phones", len(phoneToOwner),
		"addresses", len(addressToID),
		"owners", len(ownerToAddresses))
}

func (db *MemoryDB) GetOwnerByPhone(phone string) (int, []int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	info, ok := db.phoneToOwner[phone]
	if !ok {
		return 0, nil, false
	}
	return info.OwnerID, info.AddressIDs, true
}

func (db *MemoryDB) GetPhoneByOwnerID(ownerID int) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	phone, ok := db.ownerToPhone[ownerID]
	return phone, ok
}

func (db *MemoryDB) GetOwnerBySubscriberID(subscriberID int) (string, []int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	phone, ok := db.subscriberToPhone[subscriberID]
	if !ok {
		return "", nil, false
	}

	info, ok := db.phoneToOwner[phone]
	if !ok {
		return "", nil, false
	}

	return phone, info.AddressIDs, true
}

func (db *MemoryDB) GetAddressesByOwner(ownerID int) []int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.ownerToAddresses[ownerID]
}

func (db *MemoryDB) GetAddressByAddressID(addressID int) string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.addressIDToAddress[addressID]
}

func (db *MemoryDB) GetAddressIDByComponents(addrComp rosdomofon.AddressComponents) (int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	id, ok := db.addressToID[addrComp]
	return id, ok
}

func (db *MemoryDB) AddressBelongsToOwner(ownerID, addressID int) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	addresses, ok := db.ownerToAddresses[ownerID]
	if !ok {
		return false
	}
	for _, aid := range addresses {
		if aid == addressID {
			return true
		}
	}
	return false
}
