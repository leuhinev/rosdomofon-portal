package storage

import (
	"gorm.io/gorm"
	"log/slog"
	"time"
)

type Car struct {
	ID             int
	FlatID         int
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

func (s *Storage) CreateCar(car *Car) error {
	plateID, err := s.GetOrCreatePlateNumber(car.PlateNumber)
	if err != nil {
		return err
	}

	dbCar := &UserCar{
		FlatID:         car.FlatID,
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

	for _, photo := range car.Photos {
		if err := s.AddCarPhoto(car.ID, photo.PhotoData, photo.IsMain); err != nil {
			slog.Warn("failed to save photo", "error", err)
		}
	}

	return nil
}

func (s *Storage) GetCarsByFlatIDs(flatIDs []int) ([]Car, error) {
	if len(flatIDs) == 0 {
		return []Car{}, nil
	}

	var dbCars []UserCar
	result := s.DB.Where("flat_id IN ?", flatIDs).Order("id DESC").Find(&dbCars)
	if result.Error != nil {
		return nil, result.Error
	}

	cars := make([]Car, len(dbCars))
	for i, dbCar := range dbCars {
		var plate PlateNumber
		s.DB.First(&plate, dbCar.PlateID)

		var dbPhotos []CarPhotoDB
		s.DB.Where("car_id = ?", dbCar.ID).Find(&dbPhotos)

		photos := make([]CarPhotoDB, len(dbPhotos))
		for j, p := range dbPhotos {
			photos[j] = CarPhotoDB{
				ID:        p.ID,
				CarID:     p.CarID,
				PhotoData: p.PhotoData,
				IsMain:    p.IsMain,
			}
		}

		cars[i] = Car{
			ID:             dbCar.ID,
			FlatID:         dbCar.FlatID,
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

func (s *Storage) UpdateCar(id int, flatID int, comment string, autoOpen, notifyOnDetect, notifyOnEntry, notifyOnExit bool) error {
	result := s.DB.Model(&UserCar{}).
		Where("id = ? AND flat_id = ?", id, flatID).
		Updates(map[string]interface{}{
			"comment":          comment,
			"auto_open":        autoOpen,
			"notify_on_detect": notifyOnDetect,
			"notify_on_entry":  notifyOnEntry,
			"notify_on_exit":   notifyOnExit,
		})
	return result.Error
}

func (s *Storage) ExtendCarExpiry(id int, flatID int, additionalDays int) error {
	var car UserCar
	result := s.DB.Where("id = ? AND flat_id = ?", id, flatID).First(&car)
	if result.Error != nil {
		return result.Error
	}

	newExpiry := time.Unix(car.ExpiresAt, 0).AddDate(0, 0, additionalDays)
	result = s.DB.Model(&UserCar{}).
		Where("id = ? AND flat_id = ?", id, flatID).
		Update("expires_at", newExpiry.Unix())
	return result.Error
}

func (s *Storage) DeleteCar(id, flatID int) error {
	s.DB.Where("car_id = ?", id).Delete(&CarPhotoDB{})
	result := s.DB.Where("id = ? AND flat_id = ?", id, flatID).Delete(&UserCar{})
	return result.Error
}

func (s *Storage) CarBelongsToFlat(carID, flatID int) bool {
	var count int64
	s.DB.Model(&UserCar{}).Where("id = ? AND flat_id = ?", carID, flatID).Count(&count)
	return count > 0
}

func (s *Storage) AddCarPhoto(carID int, photoData string, isMain bool) error {
	photo := &CarPhotoDB{
		CarID:     carID,
		PhotoData: photoData,
		IsMain:    isMain,
	}

	if isMain {
		s.DB.Model(&CarPhotoDB{}).Where("car_id = ?", carID).Update("is_main", false)
	}

	return s.DB.Create(photo).Error
}

func (s *Storage) GetCarPhotos(carID int) ([]CarPhotoDB, error) {
	var photos []CarPhotoDB
	result := s.DB.Where("car_id = ?", carID).Order("is_main DESC").Find(&photos)
	if result.Error != nil {
		return nil, result.Error
	}
	return photos, nil
}

func (s *Storage) UpdateCarPhoto(photoID, carID int, photoData string, isMain bool) error {
	if isMain {
		s.DB.Model(&CarPhotoDB{}).Where("car_id = ?", carID).Update("is_main", false)
	}

	result := s.DB.Model(&CarPhotoDB{}).
		Where("id = ? AND car_id = ?", photoID, carID).
		Updates(map[string]interface{}{
			"photo_data": photoData,
			"is_main":    isMain,
		})
	return result.Error
}

func (s *Storage) DeleteCarPhoto(photoID, carID int) error {
	result := s.DB.Where("id = ? AND car_id = ?", photoID, carID).Delete(&CarPhotoDB{})
	return result.Error
}

// Добавьте этот метод в storage/cars.go
func (s *Storage) IsCarExists(flatID int, plateNumber string) (bool, error) {
	var plate PlateNumber
	result := s.DB.Where("plate_number = ?", plateNumber).First(&plate)
	if result.Error != nil {
		// Если номер не найден, значит дубликата нет
		if result.Error == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, result.Error
	}

	// Проверяем, есть ли автомобиль с таким plate_id и flat_id
	var count int64
	s.DB.Model(&UserCar{}).Where("flat_id = ? AND plate_id = ?", flatID, plate.ID).Count(&count)
	return count > 0, nil
}
