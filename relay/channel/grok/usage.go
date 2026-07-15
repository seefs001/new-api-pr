package grok

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const maxCLIResponseBytes = 1 << 20

type BillingUsageWindow struct {
	StatusCode int    `json:"status_code"`
	Data       any    `json:"data,omitempty"`
	Error      string `json:"error,omitempty"`
}

type BillingUsage struct {
	Weekly  BillingUsageWindow `json:"weekly"`
	Monthly BillingUsageWindow `json:"monthly"`
	Partial bool               `json:"partial"`
}

func FetchBillingUsage(ctx context.Context, client *http.Client, baseURL string, accessToken string) (BillingUsage, error) {
	if client == nil {
		return BillingUsage{}, fmt.Errorf("nil http client")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return BillingUsage{}, fmt.Errorf("empty baseURL")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return BillingUsage{}, fmt.Errorf("empty accessToken")
	}

	var usage BillingUsage
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		usage.Weekly = fetchBillingWindow(ctx, client, baseURL+"/v1/billing?format=credits", accessToken)
	}()
	go func() {
		defer wg.Done()
		usage.Monthly = fetchBillingWindow(ctx, client, baseURL+"/v1/billing", accessToken)
	}()
	wg.Wait()

	weeklyOK := usage.Weekly.Error == "" && usage.Weekly.StatusCode >= http.StatusOK && usage.Weekly.StatusCode < http.StatusMultipleChoices
	monthlyOK := usage.Monthly.Error == "" && usage.Monthly.StatusCode >= http.StatusOK && usage.Monthly.StatusCode < http.StatusMultipleChoices
	usage.Partial = weeklyOK != monthlyOK
	return usage, nil
}

func fetchBillingWindow(ctx context.Context, client *http.Client, url string, accessToken string) BillingUsageWindow {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return BillingUsageWindow{Error: err.Error()}
	}
	setCLIIdentityHeaders(req.Header, accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return BillingUsageWindow{Error: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCLIResponseBytes+1))
	if err != nil {
		return BillingUsageWindow{StatusCode: resp.StatusCode, Error: err.Error()}
	}
	if len(body) > maxCLIResponseBytes {
		return BillingUsageWindow{StatusCode: resp.StatusCode, Error: "response body exceeds 1 MiB"}
	}

	var payload any
	if len(body) > 0 {
		if err := common.Unmarshal(body, &payload); err != nil {
			payload = string(body)
		}
	}
	return BillingUsageWindow{StatusCode: resp.StatusCode, Data: payload}
}
