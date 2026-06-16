package mqtt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"rosdomofon-portal/internal/config"
	"rosdomofon-portal/internal/doors"
	"rosdomofon-portal/internal/memorydb"
	"rosdomofon-portal/internal/rosdomofon"
	"rosdomofon-portal/internal/storage"
)

type MQTTClient struct {
	client    mqtt.Client
	storage   *storage.Storage
	memoryDB  *memorydb.MemoryDB
	doorsMap  map[string]config.Door
	cfg       config.MQTTConfig
	rosClient *rosdomofon.Client
}

// PlateEvent структура для парсинга события распознавания номера
type PlateEvent struct {
	EventType string `json:"EventType"`
	EventName string `json:"EventName"`
	TimeUTC   string `json:"TimeUTC"`
	Smd       struct {
		CamType string `json:"CamType"` // in / out
	} `json:"Smd"`
	Track struct {
		ForPN struct {
			PlateNumber string `json:"PlateNumber"`
		} `json:"ForPN"`
	} `json:"Track"`
	Result struct {
		Plates []struct {
			PlateNumber string `json:"PlateNumber"`
		} `json:"Plates"`
	} `json:"Result"`
	JpegImage string `json:"JpegImage"`
}

func NewMQTTClient(cfg config.MQTTConfig, storage *storage.Storage, memoryDB *memorydb.MemoryDB, doorsMap map[string]config.Door, rosClient *rosdomofon.Client) *MQTTClient {
	return &MQTTClient{
		cfg:       cfg,
		storage:   storage,
		memoryDB:  memoryDB,
		doorsMap:  doorsMap,
		rosClient: rosClient,
	}
}

func (m *MQTTClient) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(m.cfg.Broker)
	opts.SetClientID(m.cfg.ClientID)
	if m.cfg.Username != "" {
		opts.SetUsername(m.cfg.Username)
		opts.SetPassword(m.cfg.Password)
	}
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectTimeout(30 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetOnConnectHandler(m.onConnect)
	opts.SetConnectionLostHandler(m.onConnectionLost)

	if strings.HasPrefix(m.cfg.Broker, "ssl://") || strings.HasPrefix(m.cfg.Broker, "tls://") {
		opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}

	m.client = mqtt.NewClient(opts)
	token := m.client.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	slog.Info("MQTT connected", "broker", m.cfg.Broker)
	return nil
}

func (m *MQTTClient) onConnect(client mqtt.Client) {
	slog.Info("MQTT connected, subscribing to topic", "topic", m.cfg.Topic)
	token := client.Subscribe(m.cfg.Topic, m.cfg.QOS, m.messageHandler)
	if token.Wait() && token.Error() != nil {
		slog.Error("Failed to subscribe", "error", token.Error())
	}
}

func (m *MQTTClient) onConnectionLost(client mqtt.Client, err error) {
	slog.Error("MQTT connection lost", "error", err)
}

func (m *MQTTClient) messageHandler(client mqtt.Client, msg mqtt.Message) {
	slog.Debug("Received MQTT message", "topic", msg.Topic(), "size_bytes", len(msg.Payload()))
	go m.processMessage(msg.Topic(), msg.Payload())
}

func (m *MQTTClient) processMessage(topic string, payload []byte) {
	var event PlateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		slog.Error("Failed to parse MQTT message", "error", err)
		return
	}

	if event.EventType != "PlateNumberRecognition" {
		slog.Debug("Ignoring non-plate event", "event_type", event.EventType)
		return
	}

	plateNumber := event.Track.ForPN.PlateNumber
	if plateNumber == "" && len(event.Result.Plates) > 0 {
		plateNumber = event.Result.Plates[0].PlateNumber
	}
	if plateNumber == "" {
		slog.Warn("No plate number in event")
		return
	}

	plateNumber = normalizePlate(plateNumber)

	var doorID string
	var doorCfg config.Door
	for id, d := range m.doorsMap {
		if d.MqttTopic == topic {
			doorID = id
			doorCfg = d
			break
		}
	}
	if doorID == "" {
		slog.Warn("No door configured for MQTT topic", "topic", topic)
		return
	}

	slog.Info("Processing plate event",
		"door_id", doorID,
		"plate", plateNumber,
		"event_name", event.EventName,
		"cam_type", event.Smd.CamType,
		"image_size", len(event.JpegImage),
		"free_out", doorCfg.FreeOut)

	// Если включён свободный выезд и событие detect - открываем дверь для любого номера
	if doorCfg.FreeOut && event.EventName == "detect" {
		slog.Info("Free out enabled, opening door for any plate", "door_id", doorID)
		m.openDoor(doorCfg, doorID)
		// Продолжаем обработку для отправки уведомлений, если номер есть в базе
	}

	// Получаем все записи автомобилей с этим номером
	cars, err := m.storage.GetCarsByPlateNumber(plateNumber)
	if err != nil {
		slog.Error("Failed to get cars by plate", "error", err)
		return
	}

	if len(cars) == 0 {
		slog.Debug("Plate not found in database", "plate", plateNumber)
		return
	}

	// Обрабатываем события для найденных машин
	for _, car := range cars {
		switch event.EventName {
		case "detect":
			// auto_open срабатывает только если free_out выключена (иначе дверь уже открыта)
			if !doorCfg.FreeOut && car.AutoOpen {
				slog.Info("Auto open detected, opening door", "door_id", doorID, "plate", plateNumber, "address_id", car.AddressID)
				m.openDoor(doorCfg, doorID)
			}
			// Уведомления отправляем всегда, если включены
			if car.NotifyOnDetect {
				m.sendNotification(car.AddressID, fmt.Sprintf("📸 автомобиль %s (%s)", plateNumber, car.Comment))
			}

		case "endTrack":
			camType := event.Smd.CamType
			if camType == "in" && car.NotifyOnEntry {
				m.sendNotification(car.AddressID, fmt.Sprintf("⬇️ Автомобиль %s (%s) въехал", plateNumber, car.Comment))
			} else if camType == "out" && car.NotifyOnExit {
				m.sendNotification(car.AddressID, fmt.Sprintf("⬆️️️ Автомобиль %s (%s) выехал", plateNumber, car.Comment))
			}
		}
	}
}

func (m *MQTTClient) openDoor(doorCfg config.Door, doorID string) {
	d := doors.Door{
		URL:         doorCfg.URL,
		Method:      doorCfg.Method,
		AuthType:    doorCfg.AuthType,
		Username:    doorCfg.Username,
		Password:    doorCfg.Password,
		Body:        doorCfg.Body,
		ContentType: doorCfg.ContentType,
		InsecureTLS: doorCfg.InsecureTLS,
	}

	resp, err := doors.ProxyRequest(d)
	if err != nil {
		slog.Error("Failed to open door", "door_id", doorID, "error", err)
		return
	}
	defer resp.Body.Close()
	slog.Info("Door opened", "door_id", doorID, "status", resp.StatusCode)
}

func (m *MQTTClient) sendNotification(addressID int, message string) {
	ownerID, ok := m.memoryDB.GetOwnerByAddressID(addressID)
	if !ok {
		slog.Warn("No owner found for address", "address_id", addressID)
		return
	}
	phoneStr, ok := m.memoryDB.GetPhoneByOwnerID(ownerID)
	if !ok {
		slog.Warn("No phone for owner", "owner_id", ownerID)
		return
	}
	// Убираем +7 для преобразования в int64
	phoneStr = strings.TrimPrefix(phoneStr, "+")
	phoneInt, err := strconv.ParseInt(phoneStr, 10, 64)
	if err != nil {
		slog.Error("Failed to parse phone number", "phone", phoneStr, "error", err)
		return
	}
	// Отправляем уведомление через РосДомофон
	if err := m.rosClient.SendNotification(phoneInt, message); err != nil {
		slog.Error("Failed to send notification", "phone", phoneStr, "error", err)
	} else {
		slog.Info("Notification sent", "phone", phoneStr, "message", message)
	}
}

func normalizePlate(plate string) string {
	translit := map[rune]rune{
		'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H',
		'О': 'O', 'Р': 'P', 'С': 'C', 'Т': 'T', 'У': 'Y', 'Х': 'X',
		'а': 'a', 'в': 'b', 'е': 'e', 'к': 'k', 'м': 'm', 'н': 'h',
		'о': 'o', 'р': 'p', 'с': 'c', 'т': 't', 'у': 'y', 'х': 'x',
	}
	res := strings.Map(func(r rune) rune {
		if val, ok := translit[r]; ok {
			return val
		}
		return r
	}, plate)
	return strings.ToUpper(res)
}
