package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	oauth2proxy "github.com/oauth2-proxy/oauth2-proxy/v7"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/validation"
	"github.com/obot-platform/providers/auth-providers-common/pkg/env"
	"github.com/obot-platform/providers/auth-providers-common/pkg/state"
	"github.com/obot-platform/providers/generic-oauth-auth-provider/pkg/profile"
)

const (
	defaultProviderName = "Custom OAuth"
	defaultScope        = "openid email profile"
)

type Options struct {
	ClientID                 string `env:"OBOT_GENERIC_OAUTH_AUTH_PROVIDER_CLIENT_ID"`
	ClientSecret             string `env:"OBOT_GENERIC_OAUTH_AUTH_PROVIDER_CLIENT_SECRET"`
	Issuer                   string `env:"OBOT_GENERIC_OAUTH_AUTH_PROVIDER_ISSUER"`
	ProviderName             string `env:"OBOT_GENERIC_OAUTH_AUTH_PROVIDER_NAME" default:"Custom OAuth"`
	Scope                    string `env:"OBOT_GENERIC_OAUTH_AUTH_PROVIDER_SCOPE" default:"openid email profile"`
	ObotServerURL            string `env:"OBOT_SERVER_PUBLIC_URL,OBOT_SERVER_URL"`
	PostgresConnectionDSN    string `env:"OBOT_AUTH_PROVIDER_POSTGRES_CONNECTION_DSN" optional:"true"`
	AuthCookieSecret         string `usage:"Secret used to encrypt cookie" env:"OBOT_AUTH_PROVIDER_COOKIE_SECRET"`
	AuthEmailDomains         string `usage:"Email domains allowed for authentication" default:"*" env:"OBOT_AUTH_PROVIDER_EMAIL_DOMAINS"`
	AuthTokenRefreshDuration string `usage:"Duration to refresh auth token after" optional:"true" default:"1h" env:"OBOT_AUTH_PROVIDER_TOKEN_REFRESH_DURATION"`
	LoggingEnabled           string `usage:"Enable oauth2-proxy logging" optional:"true" env:"OBOT_AUTH_PROVIDER_ENABLE_LOGGING"`
}

func main() {
	var opts Options
	if err := env.LoadEnvForStruct(&opts); err != nil {
		fmt.Printf("ERROR: generic-oauth-auth-provider: failed to load options: %v\n", err)
		os.Exit(1)
	}

	oauthProxyOpts, err := buildOAuthProxyOptions(opts)
	if err != nil {
		fmt.Printf("ERROR: generic-oauth-auth-provider: failed to build options: %v\n", err)
		os.Exit(1)
	}

	if err = validation.Validate(oauthProxyOpts); err != nil {
		fmt.Printf("ERROR: generic-oauth-auth-provider: failed to validate options: %v\n", err)
		os.Exit(1)
	}

	userInfoURL, err := discoverUserInfoEndpoint(context.Background(), opts.Issuer)
	if err != nil {
		fmt.Printf("ERROR: generic-oauth-auth-provider: failed to discover userinfo endpoint: %v\n", err)
		os.Exit(1)
	}

	oauthProxy, err := oauth2proxy.NewOAuthProxy(oauthProxyOpts, oauth2proxy.NewValidator(oauthProxyOpts.EmailDomains, oauthProxyOpts.AuthenticatedEmailsFile))
	if err != nil {
		fmt.Printf("ERROR: generic-oauth-auth-provider: failed to create oauth2 proxy: %v\n", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9999"
	}
	listenHost := os.Getenv("OBOT_PROVIDER_LISTEN_HOST")
	if listenHost == "" {
		listenHost = "127.0.0.1"
	}
	addr := net.JoinHostPort(listenHost, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Write(fmt.Appendf(nil, "http://%s", addr))
	})
	mux.HandleFunc("/obot-get-state", getState(oauthProxy, normalizedIssuer(opts.Issuer), userInfoURL))
	mux.HandleFunc("/obot-get-user-info", func(w http.ResponseWriter, r *http.Request) {
		userInfo, err := profile.FetchUserInfo(r.Context(), r.Header.Get("Authorization"), userInfoURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch user info: %v", err), http.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(userInfo)
	})
	mux.HandleFunc("/obot-list-user-auth-groups", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", oauthProxy.ServeHTTP)

	fmt.Printf("listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("ERROR: generic-oauth-auth-provider: failed to listen and serve: %v\n", err)
		os.Exit(1)
	}
}

func buildOAuthProxyOptions(opts Options) (*options.Options, error) {
	refreshDuration, err := time.ParseDuration(opts.AuthTokenRefreshDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token refresh duration: %v", err)
	}
	if refreshDuration < 0 {
		return nil, fmt.Errorf("token refresh duration must be greater than 0")
	}

	cookieSecret, err := base64.StdEncoding.DecodeString(opts.AuthCookieSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cookie secret: %v", err)
	}

	scope := strings.TrimSpace(opts.Scope)
	if scope == "" {
		scope = defaultScope
	}
	providerName := strings.TrimSpace(opts.ProviderName)
	if providerName == "" {
		providerName = defaultProviderName
	}

	legacyOpts := options.NewLegacyOptions()
	legacyOpts.LegacyProvider.ProviderType = "oidc"
	legacyOpts.LegacyProvider.ProviderName = providerName
	legacyOpts.LegacyProvider.ClientID = opts.ClientID
	legacyOpts.LegacyProvider.ClientSecret = opts.ClientSecret
	legacyOpts.LegacyProvider.OIDCIssuerURL = normalizedIssuer(opts.Issuer)
	legacyOpts.LegacyProvider.Scope = scope
	legacyOpts.LegacyProvider.CodeChallengeMethod = "S256"
	legacyOpts.LegacyProvider.InsecureOIDCSkipNonce = false
	legacyOpts.LegacyProvider.InsecureOIDCSkipIssuerVerification = false
	legacyOpts.LegacyProvider.InsecureOIDCAllowUnverifiedEmail = true
	legacyOpts.LegacyProvider.ApprovalPrompt = ""

	oauthProxyOpts, err := legacyOpts.ToOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to convert legacy options to new options: %v", err)
	}

	oauthProxyOpts.Server.BindAddress = ""
	oauthProxyOpts.MetricsServer.BindAddress = ""
	if opts.PostgresConnectionDSN != "" {
		oauthProxyOpts.Session.Type = options.PostgresSessionStoreType
		oauthProxyOpts.Session.Postgres.ConnectionDSN = opts.PostgresConnectionDSN
		oauthProxyOpts.Session.Postgres.TableNamePrefix = "generic_oauth_"
	}
	oauthProxyOpts.Cookie.Refresh = refreshDuration
	oauthProxyOpts.Cookie.Name = "obot_access_token"
	oauthProxyOpts.Cookie.Secret = string(bytes.TrimSpace(cookieSecret))
	oauthProxyOpts.Cookie.Secure = strings.HasPrefix(opts.ObotServerURL, "https://")
	oauthProxyOpts.Cookie.CSRFExpire = 30 * time.Minute
	oauthProxyOpts.RawRedirectURL = strings.TrimRight(opts.ObotServerURL, "/") + "/"
	if opts.AuthEmailDomains != "" {
		emailDomains := strings.Split(opts.AuthEmailDomains, ",")
		for i := range emailDomains {
			emailDomains[i] = strings.TrimSpace(emailDomains[i])
		}
		oauthProxyOpts.EmailDomains = emailDomains
	}

	loggingEnabled := strings.EqualFold(opts.LoggingEnabled, "true")
	oauthProxyOpts.Logging.RequestEnabled = loggingEnabled
	oauthProxyOpts.Logging.AuthEnabled = loggingEnabled
	oauthProxyOpts.Logging.StandardEnabled = loggingEnabled

	return oauthProxyOpts, nil
}

func getState(p *oauth2proxy.OAuthProxy, issuer, userInfoURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var sr state.SerializableRequest
		if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
			http.Error(w, fmt.Sprintf("failed to decode request body: %v", err), http.StatusBadRequest)
			return
		}

		reqObj, err := http.NewRequest(sr.Method, sr.URL, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create request object: %v", err), http.StatusBadRequest)
			return
		}
		reqObj.Header = sr.Header

		ss, err := state.GetSerializableState(p, reqObj)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get state: %v", err), http.StatusInternalServerError)
			fmt.Printf("ERROR: generic-oauth-auth-provider: failed to get state: %v\n", err)
			return
		}

		userInfo, err := profile.FetchUserInfo(r.Context(), fmt.Sprintf("Bearer %s", ss.AccessToken), userInfoURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get user info: %v", err), http.StatusInternalServerError)
			fmt.Printf("ERROR: generic-oauth-auth-provider: failed to get user info: %v\n", err)
			return
		}

		ss.User = userInfo.Subject
		ss.Email = userInfo.Email
		ss.EmailVerified = userInfo.EmailVerified
		ss.Issuer = issuer
		ss.PreferredUsername = userInfo.PreferredUsername
		if ss.PreferredUsername == "" {
			ss.PreferredUsername = userInfo.Name
		}
		if ss.PreferredUsername == "" {
			ss.PreferredUsername = userInfo.Email
		}

		if err := json.NewEncoder(w).Encode(ss); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode state: %v", err), http.StatusInternalServerError)
			fmt.Printf("ERROR: generic-oauth-auth-provider: failed to encode state: %v\n", err)
			return
		}
	}
}

func normalizedIssuer(issuer string) string {
	return strings.TrimRight(strings.TrimSpace(issuer), "/")
}

func discoverUserInfoEndpoint(ctx context.Context, issuer string) (string, error) {
	discoveryURL := normalizedIssuer(issuer) + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("discovery returned status %d: %s", resp.StatusCode, body)
	}

	var discovery struct {
		UserInfoEndpoint string `json:"userinfo_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return "", err
	}
	if discovery.UserInfoEndpoint == "" {
		return "", fmt.Errorf("discovery response is missing userinfo_endpoint")
	}

	return discovery.UserInfoEndpoint, nil
}
