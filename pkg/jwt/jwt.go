package jwt

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func getSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return nil, errors.New("JWT_SECRET is not set")
	}
	return []byte(secret), nil
}

func getRefreshSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_REFRESH_SECRET"))
	if secret == "" {
		return nil, errors.New("JWT_REFRESH_SECRET is not set")
	}
	return []byte(secret), nil
}

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateJWT(email string) (string, error) {
	jwtKey, err := getSecret()
	if err != nil {
		return "", err
	}

	expirationTime := time.Now().Add(15 * time.Minute)

	claims := &Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func GenerateRefreshToken(email string) (string, error) {
	refreshKey, err := getRefreshSecret()
	if err != nil {
		return "", err
	}

	expirationTime := time.Now().Add(7 * 24 * time.Hour)

	claims := &RefreshClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(refreshKey)
}

func ValidateJWT(tokenStr string) (string, error) {
	jwtKey, err := getSecret()
	if err != nil {
		return "", err
	}

	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("invalid access token")
	}

	if claims.Email == "" {
		return "", errors.New("no email in token")
	}

	return claims.Email, nil
}

func ValidateRefreshToken(tokenStr string) (string, error) {
	refreshKey, err := getRefreshSecret()
	if err != nil {
		return "", err
	}

	claims := &RefreshClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return refreshKey, nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	if claims.Email == "" {
		return "", errors.New("no email in token")
	}

	return claims.Email, nil
}
