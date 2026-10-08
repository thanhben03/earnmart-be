package service

import (
	"testing"
	"time"
)

func TestTokenManagerAccessLifecycle(t *testing.T) {
	manager, err := NewTokenManager("01234567890123456789012345678901", 15*time.Minute, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	token, expiresAt, err := manager.IssueAccessToken("user-1", "session-1", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.ParseAccessToken(token, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.SessionID != "session-1" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if !expiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("unexpected expiry: %v", expiresAt)
	}
	if _, err := manager.ParseAccessToken(token, expiresAt); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestTokenManagerRejectsTamperingAndRotatesRefreshSecrets(t *testing.T) {
	manager, _ := NewTokenManager("01234567890123456789012345678901", 15*time.Minute, 30*24*time.Hour)
	now := time.Now().UTC()
	access, _, _ := manager.IssueAccessToken("user-1", "session-1", now)
	if _, err := manager.ParseAccessToken(access+"x", now); err == nil {
		t.Fatal("expected tampered token to fail")
	}
	first, firstHash, _, _ := manager.IssueRefreshToken("session-1", now)
	second, secondHash, _, _ := manager.IssueRefreshToken("session-1", now)
	if first == second || firstHash == secondHash {
		t.Fatal("refresh tokens must be unique")
	}
	if sessionID, _ := ParseRefreshSessionID(first); sessionID != "session-1" {
		t.Fatalf("unexpected session id: %s", sessionID)
	}
}
