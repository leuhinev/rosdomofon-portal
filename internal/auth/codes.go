package auth

import (
	"crypto/rand"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"math/big"
	"time"
)

type CodeManager struct {
	mc *memcache.Client
}

func NewCodeManager(mcAddress string) *CodeManager {
	return &CodeManager{
		mc: memcache.New(mcAddress),
	}
}

func (m *CodeManager) GenerateCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

func (m *CodeManager) SaveCode(phone, code string) error {
	return m.mc.Set(&memcache.Item{
		Key:        "code:" + phone,
		Value:      []byte(code),
		Expiration: 300,
	})
}

func (m *CodeManager) VerifyCode(phone, code string) (bool, error) {
	item, err := m.mc.Get("code:" + phone)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return false, nil
		}
		return false, err
	}
	if string(item.Value) != code {
		m.incrementAttempts(phone)
		return false, nil
	}
	m.mc.Delete("code:" + phone)
	m.resetAttempts(phone)
	return true, nil
}

func (m *CodeManager) CanSendCode(phone string) bool {
	item, err := m.mc.Get("last_sent:" + phone)
	if err != nil {
		return true
	}
	lastSent := string(item.Value)
	sentTime, _ := time.Parse(time.RFC3339, lastSent)
	return time.Since(sentTime) > time.Minute
}

func (m *CodeManager) RecordSend(phone string) error {
	return m.mc.Set(&memcache.Item{
		Key:        "last_sent:" + phone,
		Value:      []byte(time.Now().Format(time.RFC3339)),
		Expiration: 60,
	})
}

func (m *CodeManager) incrementAttempts(phone string) {
	item, err := m.mc.Get("attempts:" + phone)
	attempts := 1
	if err == nil {
		attempts = int(item.Value[0]) + 1
	}
	if attempts >= 3 {
		m.mc.Set(&memcache.Item{
			Key:        "blocked:" + phone,
			Value:      []byte("1"),
			Expiration: 300,
		})
		m.mc.Delete("code:" + phone)
	} else {
		m.mc.Set(&memcache.Item{
			Key:        "attempts:" + phone,
			Value:      []byte{byte(attempts)},
			Expiration: 300,
		})
	}
}

func (m *CodeManager) resetAttempts(phone string) {
	m.mc.Delete("attempts:" + phone)
}

func (m *CodeManager) IsBlocked(phone string) bool {
	_, err := m.mc.Get("blocked:" + phone)
	return err == nil
}
