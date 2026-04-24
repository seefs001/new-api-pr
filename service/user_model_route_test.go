package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResolveUserModelRoute_FirstStrategyUsesTargetModels(t *testing.T) {
	settings := dto.UserSetting{
		ModelRouteRules: []dto.UserModelRouteRule{
			{
				ID:           "rule-1",
				Name:         "Responses auto",
				Enabled:      true,
				SourceModels: []string{"auto", "smart"},
				Endpoints:    []string{"/v1/responses", "/v1/responses/compact"},
				TargetModels: []string{"gpt-5.5", "gpt-5.5-mini"},
				Strategy:     "first",
			},
		},
	}

	result, matched := ResolveUserModelRoute(settings, "auto", "/v1/responses", 0.42)

	require.True(t, matched)
	require.Equal(t, "gpt-5.5", result.TargetModel)
	require.Equal(t, "auto", result.SourceModel)
	require.Equal(t, "/v1/responses", result.Endpoint)
	require.Equal(t, "rule-1", result.RuleID)
	require.Equal(t, "Responses auto", result.RuleName)
	require.Equal(t, "first", result.Strategy)
}

func TestResolveUserModelRoute_WeightedStrategyUsesTargets(t *testing.T) {
	settings := dto.UserSetting{
		ModelRouteRules: []dto.UserModelRouteRule{
			{
				ID:           "rule-1",
				Enabled:      true,
				SourceModels: []string{"auto"},
				Endpoints:    []string{"/v1/responses"},
				Targets: []dto.UserModelRouteTarget{
					{Model: "gpt-5.5", Weight: 80},
					{Model: "gpt-5.5-mini", Weight: 20},
				},
				Strategy: "weighted",
			},
		},
	}

	first, matched := ResolveUserModelRoute(settings, "auto", "/v1/responses", 0.79)
	require.True(t, matched)
	require.Equal(t, "gpt-5.5", first.TargetModel)

	second, matched := ResolveUserModelRoute(settings, "auto", "/v1/responses", 0.80)
	require.True(t, matched)
	require.Equal(t, "gpt-5.5-mini", second.TargetModel)
}

func TestResolveUserModelRoute_RandomStrategyUsesTargetList(t *testing.T) {
	settings := dto.UserSetting{
		ModelRouteRules: []dto.UserModelRouteRule{
			{
				ID:           "rule-1",
				Enabled:      true,
				SourceModels: []string{"auto"},
				Endpoints:    []string{"/v1/responses"},
				TargetModels: []string{"gpt-5.5", "gpt-5.5-mini", "gpt-5.5-nano"},
				Strategy:     "random",
			},
		},
	}

	result, matched := ResolveUserModelRoute(settings, "auto", "/v1/responses", 0.67)

	require.True(t, matched)
	require.Equal(t, "gpt-5.5-nano", result.TargetModel)
}

func TestResolveUserModelRoute_SkipsDisabledAndNonMatchingRules(t *testing.T) {
	settings := dto.UserSetting{
		ModelRouteRules: []dto.UserModelRouteRule{
			{
				ID:           "disabled",
				Enabled:      false,
				SourceModels: []string{"auto"},
				Endpoints:    []string{"/v1/responses"},
				TargetModels: []string{"gpt-5.5"},
				Strategy:     "first",
			},
			{
				ID:           "chat",
				Enabled:      true,
				SourceModels: []string{"auto"},
				Endpoints:    []string{"/v1/chat/completions"},
				TargetModels: []string{"gpt-5.5-mini"},
				Strategy:     "first",
			},
			{
				ID:           "responses",
				Enabled:      true,
				SourceModels: []string{"auto"},
				Endpoints:    []string{"/v1/responses"},
				TargetModels: []string{"gpt-5.5"},
				Strategy:     "first",
			},
		},
	}

	result, matched := ResolveUserModelRoute(settings, "auto", "/v1/responses?foo=bar", 0)

	require.True(t, matched)
	require.Equal(t, "responses", result.RuleID)
	require.Equal(t, "gpt-5.5", result.TargetModel)
}

func TestResolveUserModelRoute_ReturnsNoMatchWhenModelDiffers(t *testing.T) {
	settings := dto.UserSetting{
		ModelRouteRules: []dto.UserModelRouteRule{
			{
				ID:           "rule-1",
				Enabled:      true,
				SourceModels: []string{"auto"},
				Endpoints:    []string{"/v1/responses"},
				TargetModels: []string{"gpt-5.5"},
				Strategy:     "first",
			},
		},
	}

	result, matched := ResolveUserModelRoute(settings, "gpt-4.1", "/v1/responses", 0)

	require.False(t, matched)
	require.Empty(t, result.TargetModel)
}
