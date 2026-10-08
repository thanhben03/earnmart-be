package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/earnmart/earnmart-be/internal/domain"
)

type Verifier struct {
	client    *http.Client
	audiences map[string]struct{}
}

func NewVerifier(clientIDs []string) *Verifier {
	audiences := make(map[string]struct{}, len(clientIDs))
	for _, id := range clientIDs {
		if id = strings.TrimSpace(id); id != "" {
			audiences[id] = struct{}{}
		}
	}
	return &Verifier{client: &http.Client{Timeout: 8 * time.Second}, audiences: audiences}
}

type tokenInfo struct {
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	ExpiresAt     string `json:"exp"`
}

func (v *Verifier) Verify(ctx context.Context, idToken string) (domain.GoogleIdentity, error) {
	if len(v.audiences) == 0 {
		return domain.GoogleIdentity{}, errors.New("GOOGLE_CLIENT_IDS is not configured")
	}
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.GoogleIdentity{}, err
	}
	response, err := v.client.Do(req)
	if err != nil {
		return domain.GoogleIdentity{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return domain.GoogleIdentity{}, errors.New("google token is invalid")
	}
	var info tokenInfo
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		return domain.GoogleIdentity{}, err
	}
	if _, ok := v.audiences[info.Audience]; !ok {
		return domain.GoogleIdentity{}, errors.New("google token audience is invalid")
	}
	expiresAt, err := strconv.ParseInt(info.ExpiresAt, 10, 64)
	if err != nil || expiresAt <= time.Now().Unix() {
		return domain.GoogleIdentity{}, fmt.Errorf("google token expired")
	}
	verified := strings.EqualFold(info.EmailVerified, "true")
	if info.Subject == "" || info.Email == "" || !verified {
		return domain.GoogleIdentity{}, errors.New("google identity is incomplete")
	}
	return domain.GoogleIdentity{Subject: info.Subject, Email: info.Email, EmailVerified: verified, Name: info.Name}, nil
}
