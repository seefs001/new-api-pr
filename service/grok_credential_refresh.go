package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type GrokCredentialRefreshOptions struct {
	ResetCaches bool
}

type grokTokenRefreshFunc func(context.Context, string, string) (*GrokOAuthTokenResult, error)

var grokCredentialRefreshMu sync.Mutex

func RefreshGrokChannelCredential(ctx context.Context, channelID int, opts GrokCredentialRefreshOptions) (*dto.GrokCredential, *model.Channel, error) {
	return refreshGrokChannelCredential(ctx, channelID, opts, RefreshGrokOAuthTokenWithProxy)
}

func refreshGrokChannelCredential(ctx context.Context, channelID int, opts GrokCredentialRefreshOptions, refreshToken grokTokenRefreshFunc) (*dto.GrokCredential, *model.Channel, error) {
	grokCredentialRefreshMu.Lock()
	defer grokCredentialRefreshMu.Unlock()

	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, nil, err
	}
	if ch == nil {
		return nil, nil, errors.New("channel not found")
	}
	if ch.Type != constant.ChannelTypeGrok {
		return nil, nil, errors.New("channel type is not Grok Subscription")
	}

	storedKey := ch.Key
	credential, err := dto.ParseGrokCredential(strings.TrimSpace(storedKey))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(credential.RefreshToken) == "" {
		return nil, nil, errors.New("grok channel: refresh_token is required to refresh credential")
	}

	refreshCtx, cancel := context.WithTimeout(ctx, grokOAuthRequestTimeout)
	defer cancel()
	result, err := refreshToken(refreshCtx, credential.RefreshToken, ch.GetSetting().Proxy)
	if err != nil {
		return nil, nil, err
	}

	credential.AccessToken = result.AccessToken
	credential.RefreshToken = result.RefreshToken
	credential.Expired = result.ExpiresAt.UTC().Format(time.RFC3339)

	encoded, err := common.Marshal(credential)
	if err != nil {
		return nil, nil, err
	}
	update := model.DB.Model(&model.Channel{}).
		Where(map[string]any{"id": ch.Id, "key": storedKey}).
		Update("key", string(encoded))
	if update.Error != nil {
		return nil, nil, update.Error
	}
	if update.RowsAffected != 1 {
		return nil, nil, fmt.Errorf("grok channel credential changed while refresh was in progress")
	}

	if opts.ResetCaches {
		model.InitChannelCache()
		ResetProxyClientCache()
	}
	return credential, ch, nil
}
