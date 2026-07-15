package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/grok"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetGrokChannelUsage(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeGrok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not SuperGrok Subscription"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	credential, err := dto.ParseGrokCredential(strings.TrimSpace(ch.Key))
	if err != nil {
		common.SysError("failed to parse Grok credential: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析凭证失败，请检查渠道配置"})
		return
	}
	if strings.TrimSpace(credential.AccessToken) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "grok channel: access_token is required"})
		return
	}

	client, err := service.NewProxyHttpClient(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	usage, err := fetchGrokUsage(c.Request.Context(), client, ch.GetBaseURL(), credential.AccessToken)
	if err != nil {
		common.SysError("failed to fetch Grok usage: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取用量信息失败，请稍后重试"})
		return
	}

	if hasGrokAuthFailure(usage) && strings.TrimSpace(credential.RefreshToken) != "" {
		refreshCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		refreshed, _, refreshErr := service.RefreshGrokChannelCredential(refreshCtx, channelID, service.GrokCredentialRefreshOptions{ResetCaches: true})
		cancel()
		if refreshErr == nil {
			usage, err = fetchGrokUsage(c.Request.Context(), client, ch.GetBaseURL(), refreshed.AccessToken)
			if err != nil {
				common.SysError("failed to fetch Grok usage after refresh: " + err.Error())
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取用量信息失败，请稍后重试"})
				return
			}
		}
	}

	weeklyOK := isGrokUsageWindowOK(usage.Weekly)
	monthlyOK := isGrokUsageWindowOK(usage.Monthly)
	response := gin.H{"success": weeklyOK || monthlyOK, "message": "", "data": usage}
	if !weeklyOK && !monthlyOK {
		response["message"] = fmt.Sprintf("upstream statuses: weekly=%d, monthly=%d", usage.Weekly.StatusCode, usage.Monthly.StatusCode)
	}
	c.JSON(http.StatusOK, response)
}

func fetchGrokUsage(parent context.Context, client *http.Client, baseURL string, accessToken string) (grok.BillingUsage, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	return grok.FetchBillingUsage(ctx, client, baseURL, accessToken)
}

func hasGrokAuthFailure(usage grok.BillingUsage) bool {
	return usage.Weekly.StatusCode == http.StatusUnauthorized || usage.Weekly.StatusCode == http.StatusForbidden ||
		usage.Monthly.StatusCode == http.StatusUnauthorized || usage.Monthly.StatusCode == http.StatusForbidden
}

func isGrokUsageWindowOK(window grok.BillingUsageWindow) bool {
	return window.Error == "" && window.StatusCode >= http.StatusOK && window.StatusCode < http.StatusMultipleChoices
}
