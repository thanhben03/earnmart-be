package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type AccessClaims struct {
	Subject   string `json:"sub"`
	SessionID string `json:"sid"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) (*TokenManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 characters")
	}
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}, nil
}

func (m *TokenManager) IssueAccessToken(userID, sessionID string, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(m.accessTTL)
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, _ := json.Marshal(AccessClaims{Subject: userID, SessionID: sessionID, Role: "user", IssuedAt: now.Unix(), ExpiresAt: expiresAt.Unix()})
	unsigned := rawURL(header) + "." + rawURL(claims)
	return unsigned + "." + rawURL(m.sign([]byte(unsigned))), expiresAt, nil
}

func (m *TokenManager) ParseAccessToken(token string, now time.Time) (AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AccessClaims{}, errors.New("invalid access token")
	}
	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(signature, m.sign([]byte(unsigned))) != 1 {
		return AccessClaims{}, errors.New("invalid access token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessClaims{}, errors.New("invalid access token payload")
	}
	var claims AccessClaims
	if json.Unmarshal(payload, &claims) != nil || claims.Subject == "" || claims.SessionID == "" || now.Unix() >= claims.ExpiresAt {
		return AccessClaims{}, errors.New("expired or invalid access token")
	}
	return claims, nil
}

func (m *TokenManager) IssueRefreshToken(sessionID string, now time.Time) (string, string, time.Time, error) {
	secret, err := randomString(32)
	if err != nil {
		return "", "", time.Time{}, err
	}
	token := sessionID + "." + secret
	return token, hashValue(token), now.Add(m.refreshTTL), nil
}

func ParseRefreshSessionID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("invalid refresh token")
	}
	return parts[0], nil
}

func HashToken(value string) string { return hashValue(value) }

func SecureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func GenerateOTP() (string, error) {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(bytes[:])%1000000), nil
}

func HashOTP(otp, pepper string) string {
	h := hmac.New(sha256.New, []byte(pepper))
	_, _ = h.Write([]byte(otp))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func randomString(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func rawURL(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }

func (m *TokenManager) sign(value []byte) []byte {
	h := hmac.New(sha256.New, m.secret)
	_, _ = h.Write(value)
	return h.Sum(nil)
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func ParsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
