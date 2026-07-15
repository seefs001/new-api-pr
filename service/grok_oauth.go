package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	grokOAuthTokenURL       = "https://auth.x.ai/oauth2/token"
	grokOAuthClientID       = "b1a00492-073a-47ea-816f-4c329264a828"
	grokOAuthRequestTimeout = 20 * time.Second
	grokDefaultTokenTTL     = 6 * time.Hour
)

type GrokOAuthTokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func RefreshGrokOAuthTokenWithProxy(ctx context.Context, refreshToken string, proxyURL string) (*GrokOAuthTokenResult, error) {
	client, err := getGrokOAuthHTTPClient(proxyURL)
	if err != nil {
		return nil, err
	}
	return refreshGrokOAuthToken(ctx, client, grokOAuthTokenURL, refreshToken)
}

func refreshGrokOAuthToken(ctx context.Context, client *http.Client, tokenURL string, refreshToken string) (*GrokOAuthTokenResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("empty refresh_token")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", grokOAuthClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "grok-subscription/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("grok oauth refresh failed: status=%d", resp.StatusCode)
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := common.DecodeJson(io.LimitReader(resp.Body, 1<<20), &payload); err != nil {
		return nil, err
	}

	accessToken := strings.TrimSpace(payload.AccessToken)
	if accessToken == "" {
		return nil, errors.New("grok oauth refresh response missing access_token")
	}
	rotatedRefreshToken := strings.TrimSpace(payload.RefreshToken)
	if rotatedRefreshToken == "" {
		rotatedRefreshToken = refreshToken
	}
	ttl := time.Duration(payload.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = grokDefaultTokenTTL
	}

	return &GrokOAuthTokenResult{
		AccessToken:  accessToken,
		RefreshToken: rotatedRefreshToken,
		ExpiresAt:    time.Now().Add(ttl),
	}, nil
}

func getGrokOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	baseClient, err := GetHttpClientWithProxy(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	if baseClient == nil {
		return &http.Client{Timeout: grokOAuthRequestTimeout}, nil
	}
	clientCopy := *baseClient
	clientCopy.Timeout = grokOAuthRequestTimeout
	return &clientCopy, nil
}
