package storage

import (
	"gorm.io/gorm"
	"time"
)

type Car struct {
	ID             int
	AddressID      int
	PlateID        int
	PlateNumber    string
	Comment        string
	AutoOpen       bool
	NotifyOnDetect bool
	NotifyOnEntry  bool
	NotifyOnExit   bool
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Photos         []CarPhotoDB
}

func (s *Storage) GetOrCreatePlateNumber(plateNumber string) (int, error) {
	var plate PlateNumber
	result := s.DB.Where("plate_number = ?", plateNumber).First(&plate)
	if result.Error == nil {
		return plate.ID, nil
	}

	plate = PlateNumber{PlateNumber: plateNumber}
	result = s.DB.Create(&plate)
	if result.Error != nil {
		return 0, result.Error
	}
	return plate.ID, nil
}

// GetOrCreateAddress - создает запись адреса, если её нет, или возвращает существующую
func (s *Storage) GetOrCreateAddress(streetID, houseID, entranceID, flatNumber int, addressStr string) (int, error) {
	var addr Address
	result := s.DB.Where("street_id = ? AND house_id = ? AND entrance_id = ? AND flat_number = ?",
		streetID, houseID, entranceID, flatNumber).First(&addr)
	if result.Error == nil {
		return addr.ID, nil
	}

	addr = Address{
		StreetID:   streetID,
		HouseID:    houseID,
		EntranceID: entranceID,
		FlatNumber: flatNumber,
		AddressStr: addressStr,
	}
	result = s.DB.Create(&addr)
	if result.Error != nil {
		return 0, result.Error
	}
	return addr.ID, nil
}

func (s *Storage) CreateCar(car *Car) error {
	plateID, err := s.GetOrCreatePlateNumber(car.PlateNumber)
	if err != nil {
		return err
	}

	dbCar := &UserCar{
		AddressID:      car.AddressID,
		PlateID:        plateID,
		Comment:        car.Comment,
		AutoOpen:       car.AutoOpen,
		NotifyOnDetect: car.NotifyOnDetect,
		NotifyOnEntry:  car.NotifyOnEntry,
		NotifyOnExit:   car.NotifyOnExit,
		ExpiresAt:      car.ExpiresAt.Unix(),
	}

	result := s.DB.Create(dbCar)
	if result.Error != nil {
		return result.Error
	}

	car.ID = dbCar.ID
	car.CreatedAt = time.Unix(dbCar.CreatedAt, 0)
	car.UpdatedAt = time.Unix(dbCar.UpdatedAt, 0)
	car.PlateID = plateID

	return nil
}

func (s *Storage) GetCarsByAddressIDs(addressIDs []int) ([]Car, error) {
	if len(addressIDs) == 0 {
		return []Car{}, nil
	}

	var dbCars []UserCar
	result := s.DB.Where("address_id IN ?", addressIDs).Order("id DESC").Find(&dbCars)
	if result.Error != nil {
		return nil, result.Error
	}

	cars := make([]Car, len(dbCars))
	for i, dbCar := range dbCars {
		var plate PlateNumber
		s.DB.First(&plate, dbCar.PlateID)

		var dbPhotos []CarPhotoDB
		s.DB.Where("plate_id = ?", plate.ID).Order("is_main DESC").Find(&dbPhotos)

		photos := make([]CarPhotoDB, len(dbPhotos))
		for j, p := range dbPhotos {
			photos[j] = CarPhotoDB{
				ID:        p.ID,
				PlateID:   p.PlateID,
				PhotoData: p.PhotoData,
				IsMain:    p.IsMain,
				CreatedAt: p.CreatedAt,
			}
		}

		cars[i] = Car{
			ID:             dbCar.ID,
			AddressID:      dbCar.AddressID,
			PlateID:        dbCar.PlateID,
			PlateNumber:    plate.PlateNumber,
			Comment:        dbCar.Comment,
			AutoOpen:       dbCar.AutoOpen,
			NotifyOnDetect: dbCar.NotifyOnDetect,
			NotifyOnEntry:  dbCar.NotifyOnEntry,
			NotifyOnExit:   dbCar.NotifyOnExit,
			ExpiresAt:      time.Unix(dbCar.ExpiresAt, 0),
			CreatedAt:      time.Unix(dbCar.CreatedAt, 0),
			UpdatedAt:      time.Unix(dbCar.UpdatedAt, 0),
			Photos:         photos,
		}
	}
	return cars, nil
}

func (s *Storage) UpdateCar(id int, addressID int, comment string, autoOpen, notifyOnDetect, notifyOnEntry, notifyOnExit bool) error {
	result := s.DB.Model(&UserCar{}).
		Where("id = ? AND address_id = ?", id, addressID).
		Updates(map[string]interface{}{
			"comment":          comment,
			"auto_open":        autoOpen,
			"notify_on_detect": notifyOnDetect,
			"notify_on_entry":  notifyOnEntry,
			"notify_on_exit":   notifyOnExit,
		})
	return result.Error
}

func (s *Storage) ExtendCarExpiry(id int, addressID int, additionalDays int) error {
	var car UserCar
	result := s.DB.Where("id = ? AND address_id = ?", id, addressID).First(&car)
	if result.Error != nil {
		return result.Error
	}

	newExpiry := time.Unix(car.ExpiresAt, 0).AddDate(0, 0, additionalDays)
	result = s.DB.Model(&UserCar{}).
		Where("id = ? AND address_id = ?", id, addressID).
		Update("expires_at", newExpiry.Unix())
	return result.Error
}

func (s *Storage) DeleteCar(id, addressID int) error {
	result := s.DB.Where("id = ? AND address_id = ?", id, addressID).Delete(&UserCar{})
	return result.Error
}

func (s *Storage) CarBelongsToAddress(carID, addressID int) bool {
	var count int64
	s.DB.Model(&UserCar{}).Where("id = ? AND address_id = ?", carID, addressID).Count(&count)
	return count > 0
}

func (s *Storage) IsCarExists(addressID int, plateNumber string) (bool, error) {
	var plate PlateNumber
	result := s.DB.Where("plate_number = ?", plateNumber).First(&plate)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, result.Error
	}

	var count int64
	s.DB.Model(&UserCar{}).Where("address_id = ? AND plate_id = ?", addressID, plate.ID).Count(&count)
	return count > 0, nil
}

// Функции для работы с фото (привязка к plate_id)
func (s *Storage) AddCarPhotoByPlate(plateNumber string, photoData string, isMain bool) error {
	plateID, err := s.GetOrCreatePlateNumber(plateNumber)
	if err != nil {
		return err
	}

	photo := &CarPhotoDB{
		PlateID:   plateID,
		PhotoData: photoData,
		IsMain:    isMain,
	}

	if isMain {
		s.DB.Model(&CarPhotoDB{}).Where("plate_id = ?", plateID).Update("is_main", false)
	}

	return s.DB.Create(photo).Error
}

func (s *Storage) GetPhotosByPlateNumber(plateNumber string) ([]CarPhotoDB, error) {
	var plate PlateNumber
	result := s.DB.Where("plate_number = ?", plateNumber).First(&plate)
	if result.Error != nil {
		return nil, result.Error
	}

	var photos []CarPhotoDB
	s.DB.Where("plate_id = ?", plate.ID).Order("is_main DESC").Find(&photos)
	return photos, nil
}

func (s *Storage) DeleteCarPhotoByPlate(plateNumber string, photoID int) error {
	var plate PlateNumber
	result := s.DB.Where("plate_number = ?", plateNumber).First(&plate)
	if result.Error != nil {
		return result.Error
	}

	result = s.DB.Where("id = ? AND plate_id = ?", photoID, plate.ID).Delete(&CarPhotoDB{})
	return result.Error
}

func (s *Storage) UpdateCarPhotoByPlate(plateNumber string, photoID int, photoData string, isMain bool) error {
	var plate PlateNumber
	result := s.DB.Where("plate_number = ?", plateNumber).First(&plate)
	if result.Error != nil {
		return result.Error
	}

	if isMain {
		s.DB.Model(&CarPhotoDB{}).Where("plate_id = ?", plate.ID).Update("is_main", false)
	}

	result = s.DB.Model(&CarPhotoDB{}).
		Where("id = ? AND plate_id = ?", photoID, plate.ID).
		Updates(map[string]interface{}{
			"photo_data": photoData,
			"is_main":    isMain,
		})
	return result.Error
}
