package notifier

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"rosdomofon-portal/internal/config"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/rosdomofon"
	"rosdomofon-portal/internal/storage"
)

type Notifier struct {
	storage   *storage.Storage
	memoryDB  *memorydb.MemoryDB
	rosClient *rosdomofon.Client
	mc        *memcache.Client
	cfg       config.RosdomofonConfig
}

func NewNotifier(storage *storage.Storage, memoryDB *memorydb.MemoryDB, rosClient *rosdomofon.Client, mc *memcache.Client, cfg config.RosdomofonConfig) *Notifier {
	return &Notifier{
		storage:   storage,
		memoryDB:  memoryDB,
		rosClient: rosClient,
		mc:        mc,
		cfg:       cfg,
	}
}

// Start запускает периодическую проверку (раз в сутки)
func (n *Notifier) Start() {
	// Первый запуск через 5 секунд после старта, чтобы дать время инициализироваться
	time.Sleep(60 * time.Second)
	n.checkAndNotify()

	// Затем каждый день в 9:00 (можно настроить)
	go func() {
		for {
			now := time.Now()
			// Рассчитываем время до следующего запуска в 9:00
			next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			duration := next.Sub(now)
			slog.Info("Next expiry notification check scheduled", "time", next.Format(time.RFC3339), "in", duration)
			time.Sleep(duration)
			n.checkAndNotify()
		}
	}()
}

func (n *Notifier) checkAndNotify() {
	slog.Info("Running expiry notification check")

	days := n.cfg.ExpiryNotificationDays
	if days <= 0 {
		slog.Warn("Expiry notification days not configured, skipping")
		return
	}

	cars, err := n.storage.GetCarsExpiringSoon(days)
	if err != nil {
		slog.Error("Failed to get expiring cars", "error", err)
		return
	}

	if len(cars) == 0 {
		slog.Info("No cars expiring soon")
		return
	}

	// Группируем по address_id для отправки одного уведомления на адрес
	byAddress := make(map[int][]storage.Car)
	for _, car := range cars {
		byAddress[car.AddressID] = append(byAddress[car.AddressID], car)
	}

	today := time.Now().Format("2006-01-02")
	for addressID, carList := range byAddress {
		// Проверяем, отправляли ли уже уведомление сегодня для этого адреса
		key := fmt.Sprintf("expiry_notif:%d:%s", addressID, today)
		if _, err := n.mc.Get(key); err == nil {
			slog.Debug("Notification already sent for address today", "address_id", addressID)
			continue
		}

		// Получаем владельца адреса и телефон
		ownerID, ok := n.memoryDB.GetOwnerByAddressID(addressID)
		if !ok {
			slog.Warn("No owner for address", "address_id", addressID)
			continue
		}
		phoneStr, ok := n.memoryDB.GetPhoneByOwnerID(ownerID)
		if !ok {
			slog.Warn("No phone for owner", "owner_id", ownerID)
			continue
		}

		// Формируем сообщение: список номеров, у которых истекает срок
		var plates []string
		for _, car := range carList {
			expiresLocal := car.ExpiresAt.Local()
			plates = append(plates, fmt.Sprintf("%s (до %s %s)",
				car.PlateNumber,
				expiresLocal.Format("02.01.2006"),
				expiresLocal.Format("15:04")))
		}
		message := fmt.Sprintf("У следующих автомобилей истекает срок действия: %s", strings.Join(plates, ", "))

		// Отправляем уведомление
		phoneStr = strings.TrimPrefix(phoneStr, "+")
		phoneInt, err := strconv.ParseInt(phoneStr, 10, 64)
		if err != nil {
			slog.Error("Failed to parse phone", "phone", phoneStr, "error", err)
			continue
		}
		if err := n.rosClient.SendNotification(phoneInt, message); err != nil {
			slog.Error("Failed to send expiry notification", "address_id", addressID, "phone", phoneStr, "error", err)
			continue
		}

		// Сохраняем факт отправки в memcached (TTL на 24 часа)
		item := &memcache.Item{
			Key:        key,
			Value:      []byte("1"),
			Expiration: 86400, // 24 часа
		}
		if err := n.mc.Set(item); err != nil {
			slog.Error("Failed to store notification state in memcache", "error", err)
		} else {
			slog.Info("Expiry notification sent", "address_id", addressID, "phone", phoneStr, "cars_count", len(carList))
		}
	}
}
