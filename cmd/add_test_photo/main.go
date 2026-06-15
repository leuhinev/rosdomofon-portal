package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"github.com/disintegration/imaging"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
)

// Структуры для работы с БД (копия из основного проекта)
type CarPhotoDB struct {
	ID        int    `gorm:"primaryKey;autoIncrement"`
	CarID     int    `gorm:"not null;index"`
	PhotoData string `gorm:"type:longtext;not null"`
	IsMain    bool   `gorm:"default:false"`
	CreatedAt int64  `gorm:"autoCreateTime"`
}

func (CarPhotoDB) TableName() string {
	return "car_photos"
}

func main() {
	// Параметры командной строки
	var (
		dsn      = flag.String("dsn", "root:password@tcp(localhost:3306)/portal?parseTime=true", "MySQL DSN")
		carID    = flag.Int("car-id", 1, "ID автомобиля (user_cars.id)")
		imageURL = flag.String("image-url", "", "URL изображения")
	)
	flag.Parse()

	if *imageURL == "" {
		log.Fatal("image-url is required")
	}

	// Подключаемся к БД
	db, err := gorm.Open(mysql.Open(*dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	log.Println("Downloading image from:", *imageURL)

	// Скачиваем изображение
	resp, err := http.Get(*imageURL)
	if err != nil {
		log.Fatal("Failed to download image:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatal("Bad response status:", resp.StatusCode)
	}

	// Декодируем изображение
	img, format, err := image.Decode(resp.Body)
	if err != nil {
		log.Fatal("Failed to decode image:", err)
	}

	log.Printf("Image decoded, format: %s, size: %dx%d", format, img.Bounds().Dx(), img.Bounds().Dy())

	// Изменяем размер до 300x300
	resized := imaging.Resize(img, 300, 300, imaging.Lanczos)

	// Кодируем в base64
	var buf bytes.Buffer
	if format == "png" {
		err = png.Encode(&buf, resized)
	} else {
		err = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85})
	}
	if err != nil {
		log.Fatal("Failed to encode image:", err)
	}

	base64Image := "data:image/" + format + ";base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// Сохраняем в базу
	photo := CarPhotoDB{
		CarID:     *carID,
		PhotoData: base64Image,
		IsMain:    true,
	}

	result := db.Create(&photo)
	if result.Error != nil {
		log.Fatal("Failed to save photo:", result.Error)
	}

	log.Printf("✅ Photo saved successfully! ID: %d, CarID: %d", photo.ID, photo.CarID)
}
