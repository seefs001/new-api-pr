package grok

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func FetchModelIDs(ctx context.Context, client *http.Client, baseURL string, accessToken string) ([]string, int, error) {
	if client == nil {
		return nil, 0, fmt.Errorf("nil http client")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, 0, fmt.Errorf("empty baseURL")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, 0, fmt.Errorf("empty accessToken")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, 0, err
	}
	setCLIIdentityHeaders(req.Header, accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCLIResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(body) > maxCLIResponseBytes {
		return nil, resp.StatusCode, fmt.Errorf("response body exceeds 1 MiB")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, resp.StatusCode, nil
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, resp.StatusCode, err
	}
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, resp.StatusCode, nil
}
