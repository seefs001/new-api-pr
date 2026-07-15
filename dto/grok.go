package dto

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type GrokCredential struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expired      string `json:"expired,omitempty"`
}

func ParseGrokCredential(raw string) (*GrokCredential, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("grok channel: empty credential")
	}
	var credential GrokCredential
	if err := common.Unmarshal([]byte(raw), &credential); err != nil {
		return nil, errors.New("grok channel: invalid credential json")
	}
	return &credential, nil
}
