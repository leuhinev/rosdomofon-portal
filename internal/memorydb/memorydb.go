package memorydb

import (
	"log/slog"
	"rosdomofon-portal/internal/rosdomofon"
	"sync"
)

type MemoryDB struct {
	mu                 sync.RWMutex
	phoneToOwner       map[string]rosdomofon.OwnerInfo
	addressIDToAddress map[int]string
	addressToID        map[rosdomofon.AddressComponents]int
	ownerToAddresses   map[int][]int
	ownerToPhone       map[int]string
	subscriberToPhone  map[int]string
	addressToOwner     map[int]int // обратный индекс: address_id -> owner_id
}

func New() *MemoryDB {
	return &MemoryDB{
		phoneToOwner:       make(map[string]rosdomofon.OwnerInfo),
		addressIDToAddress: make(map[int]string),
		addressToID:        make(map[rosdomofon.AddressComponents]int),
		ownerToAddresses:   make(map[int][]int),
		ownerToPhone:       make(map[int]string),
		subscriberToPhone:  make(map[int]string),
		addressToOwner:     make(map[int]int),
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

	// address_id -> address_str
	db.addressIDToAddress = make(map[int]string)
	for addrComp, id := range addressToID {
		db.addressIDToAddress[id] = addrComp.AddressStr
	}

	// ownerToPhone и subscriberToPhone
	db.ownerToPhone = make(map[int]string)
	db.subscriberToPhone = make(map[int]string)
	for phone, info := range phoneToOwner {
		db.ownerToPhone[info.OwnerID] = phone
		db.subscriberToPhone[info.OwnerID] = phone
	}

	// addressToOwner: построить обратный индекс
	db.addressToOwner = make(map[int]int)
	for ownerID, addresses := range ownerToAddresses {
		for _, addrID := range addresses {
			db.addressToOwner[addrID] = ownerID
		}
	}

	slog.Debug("memorydb updated",
		"phones", len(phoneToOwner),
		"addresses", len(addressToID),
		"owners", len(ownerToAddresses))
}

// GetOwnerByPhone возвращает owner_id и список address_id по номеру телефона
func (db *MemoryDB) GetOwnerByPhone(phone string) (int, []int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	info, ok := db.phoneToOwner[phone]
	if !ok {
		return 0, nil, false
	}
	return info.OwnerID, info.AddressIDs, true
}

// GetPhoneByOwnerID возвращает телефон по owner_id
func (db *MemoryDB) GetPhoneByOwnerID(ownerID int) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	phone, ok := db.ownerToPhone[ownerID]
	return phone, ok
}

// GetOwnerBySubscriberID возвращает телефон и список address_id по subscriber_id
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

// GetAddressesByOwner возвращает список address_id для владельца
func (db *MemoryDB) GetAddressesByOwner(ownerID int) []int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.ownerToAddresses[ownerID]
}

// GetAddressByAddressID возвращает строку адреса по address_id
func (db *MemoryDB) GetAddressByAddressID(addressID int) string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.addressIDToAddress[addressID]
}

// GetAddressIDByComponents возвращает address_id по компонентам адреса
func (db *MemoryDB) GetAddressIDByComponents(addrComp rosdomofon.AddressComponents) (int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	id, ok := db.addressToID[addrComp]
	return id, ok
}

// GetOwnerByAddressID возвращает owner_id по address_id
func (db *MemoryDB) GetOwnerByAddressID(addressID int) (int, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	ownerID, ok := db.addressToOwner[addressID]
	return ownerID, ok
}

// AddressBelongsToOwner проверяет, принадлежит ли address_id владельцу
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
