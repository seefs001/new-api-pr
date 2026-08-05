package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAdminUserInvitationRequestPreservesExistingPlanSnapshot(t *testing.T) {
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = ""
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})

	existing := &model.InvitationPlanSnapshot{
		PlanId: 17, Title: "Promised plan", DurationUnit: model.SubscriptionDurationMonth,
		DurationValue: 3, TotalAmount: 2400, QuotaResetPeriod: model.SubscriptionResetWeekly,
		AllowWalletOverflow: false,
	}
	request := &adminUserInvitationRequest{
		Quota: 300, Group: "default", PlanId: existing.PlanId,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}

	snapshot, err := normalizeAdminUserInvitationRequest(request, existing)
	require.NoError(t, err)
	assert.Equal(t, existing, snapshot)
	assert.NotSame(t, existing, snapshot)
}

func TestRedactPendingClaimIdentityHidesInternalCredentials(t *testing.T) {
	user := &model.User{
		Id: 42, Username: "inv_internal", DisplayName: "internal", AffCode: "inv-code",
		Status: common.UserStatusPendingClaim,
	}

	redactPendingClaimIdentity(user)

	assert.Empty(t, user.Username)
	assert.Empty(t, user.DisplayName)
	assert.Empty(t, user.AffCode)
	assert.Equal(t, 42, user.Id)
}
