package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type SupabaseUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type SupabaseVerifier struct {
	URL            string
	PublishableKey string
	Client         *http.Client
}

func (v SupabaseVerifier) Verify(ctx context.Context, token string) (SupabaseUser, error) {
	if v.URL == "" || v.PublishableKey == "" {
		return SupabaseUser{}, fmt.Errorf("Supabase Auth is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(v.URL, "/")+"/auth/v1/user", nil)
	if err != nil {
		return SupabaseUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", v.PublishableKey)
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return SupabaseUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SupabaseUser{}, fmt.Errorf("Supabase rejected access token with status %d", resp.StatusCode)
	}
	var user SupabaseUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return SupabaseUser{}, err
	}
	if user.ID == "" {
		return SupabaseUser{}, fmt.Errorf("Supabase user response did not contain an id")
	}
	return user, nil
}
