package profile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUserInfoPreservesOIDCClaims(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("expected authorization header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"sub": "user-123",
			"email": "dev@example.com",
			"email_verified": true,
			"preferred_username": "dev",
			"name": "Dev User",
			"picture": "https://example.com/avatar.png"
		}`))
	}))
	defer server.Close()

	userInfo, err := FetchUserInfo(context.Background(), "Bearer access-token", server.URL)
	if err != nil {
		t.Fatalf("FetchUserInfo returned error: %v", err)
	}

	if userInfo.Subject != "user-123" {
		t.Fatalf("expected subject user-123, got %q", userInfo.Subject)
	}
	if userInfo.Email != "dev@example.com" {
		t.Fatalf("expected email, got %q", userInfo.Email)
	}
	if userInfo.EmailVerified == nil || !*userInfo.EmailVerified {
		t.Fatalf("expected email_verified true, got %#v", userInfo.EmailVerified)
	}
	if userInfo.PreferredUsername != "dev" {
		t.Fatalf("expected preferred username, got %q", userInfo.PreferredUsername)
	}
	if userInfo.Name != "Dev User" {
		t.Fatalf("expected name, got %q", userInfo.Name)
	}
	if userInfo.Picture != "https://example.com/avatar.png" {
		t.Fatalf("expected picture, got %q", userInfo.Picture)
	}
}

func TestFetchUserInfoRejectsMissingSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"email":"dev@example.com"}`))
	}))
	defer server.Close()

	_, err := FetchUserInfo(context.Background(), "Bearer access-token", server.URL)
	if err == nil {
		t.Fatalf("expected error")
	}
}
