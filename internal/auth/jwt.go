package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type Claims struct {
	OwnerID int   `json:"owner_id"`
	FlatIDs []int `json:"flat_ids"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{secret: []byte(secret)}
}

func (m *JWTManager) Generate(ownerID int, flatIDs []int) (string, error) {
	claims := &Claims{
		OwnerID: ownerID,
		FlatIDs: flatIDs,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(60 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Verify(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// Refresh - обновление токена (всегда можно обновить)
func (m *JWTManager) Refresh(tokenStr string) (string, error) {
	claims, err := m.Verify(tokenStr)
	if err != nil {
		return "", err
	}

	// Проверяем, что токен не истек
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return "", errors.New("token expired")
	}

	// Генерируем новый токен с новым сроком действия
	return m.Generate(claims.OwnerID, claims.FlatIDs)
}
