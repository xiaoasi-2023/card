package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func passwordHash(value string) (string, error) {
	result, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(result), err
}

func passwordMatches(hash, value string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value)) == nil
}

func encryptString(key, plaintext string) (string, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	data := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawStdEncoding.EncodeToString(data), nil
}

func decryptString(key, encoded string) (string, error) {
	data, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(data) < gcm.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	plaintext, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	return string(plaintext), err
}

func keyedHash(key, value string) string {
	h := hmac.New(sha256.New, []byte(key))
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// normalizeClaimCode 规范化领取码：去空白、统一大写。
func normalizeClaimCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// generateClaimCode 生成 TRAF- 前缀领取码（与上游真卡密无关）。
func generateClaimCode() (string, error) {
	raw := make([]byte, 10)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return "TRAF-" + strings.ToUpper(hex.EncodeToString(raw)), nil
}

// generateQueryPassword 生成短查单密码，便于与领取码一并灌小铺或售后使用。
func generateQueryPassword() (string, error) {
	raw := make([]byte, 5)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(raw)), nil
}

func signPayment(key string, req paymentCallbackRequest) string {
	canonical := fmt.Sprintf("%s|%s|%s|%d|%s|%d", req.MerchantID, req.OrderNo, req.TradeNo, req.AmountCents, req.Currency, req.Timestamp)
	return keyedHash(key, canonical)
}

func verifySignature(expected, actual string) bool {
	expectedBytes, err1 := hex.DecodeString(expected)
	actualBytes, err2 := hex.DecodeString(strings.TrimSpace(actual))
	return err1 == nil && err2 == nil && hmac.Equal(expectedBytes, actualBytes)
}

type authClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func issueToken(secret string, user User) (string, error) {
	now := time.Now()
	claims := authClaims{Role: user.Role, RegisteredClaims: jwt.RegisteredClaims{
		Subject:   fmt.Sprint(user.ID),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func parseToken(secret, raw string) (uint, string, error) {
	token, err := jwt.ParseWithClaims(raw, &authClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", errors.New("invalid token")
	}
	claims, ok := token.Claims.(*authClaims)
	if !ok {
		return 0, "", errors.New("invalid claims")
	}
	var id uint
	if _, err := fmt.Sscan(claims.Subject, &id); err != nil || id == 0 {
		return 0, "", errors.New("invalid subject")
	}
	return id, claims.Role, nil
}
