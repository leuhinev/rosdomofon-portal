package storage

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log/slog"
)

type Storage struct {
	DB *gorm.DB
}

// Удаляем таблицу owners - она не нужна

type UserCar struct {
	ID             int    `gorm:"primaryKey;autoIncrement"`
	FlatID         int    `gorm:"not null;index"` // Только flat_id
	PlateNumber    string `gorm:"type:varchar(15);not null;index"`
	Comment        string `gorm:"type:text"`
	AutoOpen       bool   `gorm:"default:false"`
	NotifyOnDetect bool   `gorm:"default:false"`
	NotifyOnEntry  bool   `gorm:"default:false"`
	NotifyOnExit   bool   `gorm:"default:false"`
	ExpiresAt      int64  `gorm:"not null;index"`
	CreatedAt      int64  `gorm:"autoCreateTime"`
	UpdatedAt      int64  `gorm:"autoUpdateTime"`
}

func (UserCar) TableName() string {
	return "user_cars"
}

type UserKey struct {
	ID        int    `gorm:"primaryKey;autoIncrement"`
	FlatID    int    `gorm:"not null;index"` // Только flat_id
	KeyData   string `gorm:"type:varchar(64);not null;uniqueIndex:idx_flat_key"`
	Comment   string `gorm:"type:text"`
	CreatedAt int64  `gorm:"autoCreateTime"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

func (UserKey) TableName() string {
	return "user_keys"
}

func NewMySQL(dsn string) (*Storage, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return nil, err
	}

	// Автоматическая миграция (без таблицы owners)
	err = db.AutoMigrate(&UserCar{}, &UserKey{})
	if err != nil {
		slog.Error("failed to migrate database", "error", err)
		return nil, err
	}

	slog.Info("database migrated successfully")

	return &Storage{DB: db}, nil
}

func (s *Storage) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
