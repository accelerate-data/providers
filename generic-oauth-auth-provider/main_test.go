package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildOAuthProxyOptionsConfiguresSecureOIDC(t *testing.T) {
	cookieSecret := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	opts, err := buildOAuthProxyOptions(Options{
		ClientID:                 "client-1",
		ClientSecret:             "secret-1",
		Issuer:                   "https://issuer.example.com/",
		ProviderName:             "Studio",
		ObotServerURL:            "https://obot.example.com",
		AuthCookieSecret:         cookieSecret,
		AuthEmailDomains:         "example.com, studio.example.com",
		AuthTokenRefreshDuration: "30m",
		Scope:                    "",
	})
	if err != nil {
		t.Fatalf("buildOAuthProxyOptions returned error: %v", err)
	}

	if len(opts.Providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(opts.Providers))
	}

	provider := opts.Providers[0]
	if provider.Type != "oidc" {
		t.Fatalf("expected provider type oidc, got %q", provider.Type)
	}
	if provider.Name != "Studio" {
		t.Fatalf("expected provider name Studio, got %q", provider.Name)
	}
	if provider.ClientID != "client-1" || provider.ClientSecret != "secret-1" {
		t.Fatalf("expected client credentials to be set")
	}
	if provider.Scope != "openid email profile" {
		t.Fatalf("expected default scope, got %q", provider.Scope)
	}
	if provider.CodeChallengeMethod != "S256" {
		t.Fatalf("expected PKCE S256, got %q", provider.CodeChallengeMethod)
	}
	if provider.OIDCConfig.IssuerURL != "https://issuer.example.com" {
		t.Fatalf("expected normalized issuer, got %q", provider.OIDCConfig.IssuerURL)
	}
	if provider.OIDCConfig.InsecureSkipNonce == nil || *provider.OIDCConfig.InsecureSkipNonce {
		t.Fatalf("expected nonce validation to be enabled")
	}
	if provider.OIDCConfig.InsecureSkipIssuerVerification == nil || *provider.OIDCConfig.InsecureSkipIssuerVerification {
		t.Fatalf("expected issuer validation to be enabled")
	}
	if provider.OIDCConfig.InsecureAllowUnverifiedEmail == nil || !*provider.OIDCConfig.InsecureAllowUnverifiedEmail {
		t.Fatalf("expected unverified email to be allowed through for Obot-side linking rules")
	}
	if opts.Cookie.Refresh != 30*time.Minute {
		t.Fatalf("expected 30m refresh, got %s", opts.Cookie.Refresh)
	}
	if !opts.Cookie.Secure {
		t.Fatalf("expected secure cookie for HTTPS Obot URL")
	}
	if opts.RawRedirectURL != "https://obot.example.com/" {
		t.Fatalf("expected Obot redirect URL, got %q", opts.RawRedirectURL)
	}
	if got, want := opts.EmailDomains, []string{"example.com", "studio.example.com"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("expected email domains %v, got %v", want, got)
	}
}

func TestBuildOAuthProxyOptionsRejectsNegativeRefreshDuration(t *testing.T) {
	_, err := buildOAuthProxyOptions(Options{
		ClientID:                 "client-1",
		ClientSecret:             "secret-1",
		Issuer:                   "https://issuer.example.com",
		ProviderName:             "Studio",
		ObotServerURL:            "https://obot.example.com",
		AuthCookieSecret:         base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012")),
		AuthEmailDomains:         "*",
		AuthTokenRefreshDuration: "-1m",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDiscoverUserInfoEndpoint(t *testing.T) {
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Fatalf("expected discovery path, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"issuer": "` + r.Host + `",
			"authorization_endpoint": "https://issuer.example.com/auth",
			"token_endpoint": "https://issuer.example.com/token",
			"jwks_uri": "https://issuer.example.com/jwks",
			"userinfo_endpoint": "https://issuer.example.com/userinfo"
		}`))
	}))
	defer issuer.Close()

	endpoint, err := discoverUserInfoEndpoint(t.Context(), issuer.URL)
	if err != nil {
		t.Fatalf("discoverUserInfoEndpoint returned error: %v", err)
	}
	if endpoint != "https://issuer.example.com/userinfo" {
		t.Fatalf("expected userinfo endpoint, got %q", endpoint)
	}
}

func TestDiscoverUserInfoEndpointRejectsMissingEndpoint(t *testing.T) {
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"` + r.Host + `"}`))
	}))
	defer issuer.Close()

	_, err := discoverUserInfoEndpoint(t.Context(), issuer.URL)
	if err == nil {
		t.Fatalf("expected error")
	}
}
