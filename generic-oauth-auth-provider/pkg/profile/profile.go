package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type UserInfo struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     *bool  `json:"email_verified,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Name              string `json:"name,omitempty"`
	Picture           string `json:"picture,omitempty"`
}

func FetchUserInfo(ctx context.Context, accessToken, url string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d: %s", resp.StatusCode, result)
	}

	var userInfo UserInfo
	if err = json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}
	if userInfo.Subject == "" {
		return nil, fmt.Errorf("userinfo response is missing sub")
	}

	return &userInfo, nil
}
