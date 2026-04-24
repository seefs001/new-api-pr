package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

const (
	UserModelRouteStrategyFirst    = "first"
	UserModelRouteStrategyRandom   = "random"
	UserModelRouteStrategyWeighted = "weighted"

	maxUserModelRouteRules         = 50
	maxUserModelRouteValuesPerRule = 50
)

type UserModelRouteResult struct {
	RuleID      string `json:"rule_id,omitempty"`
	RuleName    string `json:"rule_name,omitempty"`
	SourceModel string `json:"source_model,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	TargetModel string `json:"target_model,omitempty"`
	Strategy    string `json:"strategy,omitempty"`
}

func ResolveUserModelRoute(settings dto.UserSetting, sourceModel string, endpoint string, randomValue float64) (UserModelRouteResult, bool) {
	sourceModel = strings.TrimSpace(sourceModel)
	endpoint = normalizeModelRouteEndpoint(endpoint)
	if sourceModel == "" || endpoint == "" {
		return UserModelRouteResult{}, false
	}

	for _, rule := range settings.ModelRouteRules {
		if !rule.Enabled || !matchesModelRouteValue(rule.SourceModels, sourceModel) || !matchesModelRouteValue(rule.Endpoints, endpoint) {
			continue
		}
		targetModel, strategy := selectUserModelRouteTarget(rule, randomValue)
		if targetModel == "" {
			continue
		}
		return UserModelRouteResult{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			SourceModel: sourceModel,
			Endpoint:    endpoint,
			TargetModel: targetModel,
			Strategy:    strategy,
		}, true
	}
	return UserModelRouteResult{}, false
}

func ValidateUserModelRouteRules(rules []dto.UserModelRouteRule) error {
	if len(rules) > maxUserModelRouteRules {
		return fmt.Errorf("model route rules exceed limit %d", maxUserModelRouteRules)
	}
	for index, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if len(rule.SourceModels) == 0 {
			return fmt.Errorf("model route rule %d source_models is empty", index+1)
		}
		if len(rule.Endpoints) == 0 {
			return fmt.Errorf("model route rule %d endpoints is empty", index+1)
		}
		if len(rule.TargetModels) == 0 && len(rule.Targets) == 0 {
			return fmt.Errorf("model route rule %d targets is empty", index+1)
		}
		if len(rule.SourceModels) > maxUserModelRouteValuesPerRule ||
			len(rule.Endpoints) > maxUserModelRouteValuesPerRule ||
			len(rule.TargetModels) > maxUserModelRouteValuesPerRule ||
			len(rule.Targets) > maxUserModelRouteValuesPerRule {
			return fmt.Errorf("model route rule %d values exceed limit %d", index+1, maxUserModelRouteValuesPerRule)
		}
		for _, target := range rule.Targets {
			if strings.TrimSpace(target.Model) == "" {
				return fmt.Errorf("model route rule %d target model is empty", index+1)
			}
			if target.Weight < 0 {
				return fmt.Errorf("model route rule %d target weight cannot be negative", index+1)
			}
		}
	}
	return nil
}

func normalizeModelRouteEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if idx := strings.Index(endpoint, "?"); idx >= 0 {
		endpoint = endpoint[:idx]
	}
	return strings.TrimRight(endpoint, "/")
}

func matchesModelRouteValue(values []string, target string) bool {
	target = strings.TrimSpace(target)
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if value == "*" || value == target {
			return true
		}
		normalized = append(normalized, value)
	}
	return lo.Contains(normalized, target)
}

func selectUserModelRouteTarget(rule dto.UserModelRouteRule, randomValue float64) (string, string) {
	strategy := strings.TrimSpace(rule.Strategy)
	if strategy == "" {
		strategy = UserModelRouteStrategyFirst
	}

	switch strategy {
	case UserModelRouteStrategyRandom:
		return selectRandomUserModelRouteTarget(rule, randomValue), strategy
	case UserModelRouteStrategyWeighted:
		target := selectWeightedUserModelRouteTarget(rule.Targets, randomValue)
		if target != "" {
			return target, strategy
		}
		return selectFirstUserModelRouteTarget(rule), UserModelRouteStrategyFirst
	default:
		return selectFirstUserModelRouteTarget(rule), UserModelRouteStrategyFirst
	}
}

func selectFirstUserModelRouteTarget(rule dto.UserModelRouteRule) string {
	for _, model := range rule.TargetModels {
		if model = strings.TrimSpace(model); model != "" {
			return model
		}
	}
	for _, target := range rule.Targets {
		if model := strings.TrimSpace(target.Model); model != "" {
			return model
		}
	}
	return ""
}

func selectRandomUserModelRouteTarget(rule dto.UserModelRouteRule, randomValue float64) string {
	models := collectUserModelRouteTargetModels(rule)
	if len(models) == 0 {
		return ""
	}
	index := int(clampRouteRandomValue(randomValue) * float64(len(models)))
	if index >= len(models) {
		index = len(models) - 1
	}
	return models[index]
}

func selectWeightedUserModelRouteTarget(targets []dto.UserModelRouteTarget, randomValue float64) string {
	totalWeight := 0
	for _, target := range targets {
		if strings.TrimSpace(target.Model) == "" || target.Weight <= 0 {
			continue
		}
		totalWeight += target.Weight
	}
	if totalWeight <= 0 {
		return ""
	}

	threshold := clampRouteRandomValue(randomValue) * float64(totalWeight)
	accumulated := 0
	for _, target := range targets {
		model := strings.TrimSpace(target.Model)
		if model == "" || target.Weight <= 0 {
			continue
		}
		accumulated += target.Weight
		if threshold < float64(accumulated) {
			return model
		}
	}
	return ""
}

func collectUserModelRouteTargetModels(rule dto.UserModelRouteRule) []string {
	models := make([]string, 0, len(rule.TargetModels)+len(rule.Targets))
	for _, model := range rule.TargetModels {
		if model = strings.TrimSpace(model); model != "" {
			models = append(models, model)
		}
	}
	if len(models) > 0 {
		return models
	}
	for _, target := range rule.Targets {
		if model := strings.TrimSpace(target.Model); model != "" {
			models = append(models, model)
		}
	}
	return models
}

func clampRouteRandomValue(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value >= 1 {
		return 0.999999999
	}
	return value
}
