package storage

import (
	"time"
)

type Car struct {
	ID             int
	FlatID         int
	PlateNumber    string
	Comment        string
	AutoOpen       bool
	NotifyOnDetect bool
	NotifyOnEntry  bool
	NotifyOnExit   bool
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (s *Storage) CreateCar(car *Car) error {
	dbCar := &UserCar{
		FlatID:         car.FlatID,
		PlateNumber:    car.PlateNumber,
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
		cars[i] = Car{
			ID:             dbCar.ID,
			FlatID:         dbCar.FlatID,
			PlateNumber:    dbCar.PlateNumber,
			Comment:        dbCar.Comment,
			AutoOpen:       dbCar.AutoOpen,
			NotifyOnDetect: dbCar.NotifyOnDetect,
			NotifyOnEntry:  dbCar.NotifyOnEntry,
			NotifyOnExit:   dbCar.NotifyOnExit,
			ExpiresAt:      time.Unix(dbCar.ExpiresAt, 0),
			CreatedAt:      time.Unix(dbCar.CreatedAt, 0),
			UpdatedAt:      time.Unix(dbCar.UpdatedAt, 0),
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

func (s *Storage) DeleteCar(id, flatID int) error {
	result := s.DB.Where("id = ? AND flat_id = ?", id, flatID).Delete(&UserCar{})
	return result.Error
}

func (s *Storage) CarBelongsToFlat(carID, flatID int) bool {
	var count int64
	s.DB.Model(&UserCar{}).Where("id = ? AND flat_id = ?", carID, flatID).Count(&count)
	return count > 0
}
