package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useUserInvitationTestDB(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{}, &UserInvitation{}, &SubscriptionPlan{}, &UserSubscription{},
		&SubscriptionPreConsumeRecord{}, &Token{},
	))
	DB, LOG_DB = db, db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		_ = sqlDB.Close()
	})
}

func TestCreateUserInvitationCreatesPendingUserWithPromisedEntitlements(t *testing.T) {
	useUserInvitationTestDB(t)

	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7,
		Quota:     12345,
		Group:     "vip",
		Remark:    "launch partner",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, issue.LinkToken)
	require.NotEmpty(t, issue.ManualCode)

	user, err := GetUserById(issue.Invitation.UserId, true)
	require.NoError(t, err)
	assert.Equal(t, common.UserStatusPendingClaim, user.Status)
	assert.Equal(t, 12345, user.Quota)
	assert.Equal(t, "vip", user.Group)
	assert.Empty(t, user.Password)
	assert.Zero(t, user.ActivatedAt)
	assert.NotEmpty(t, user.Username)
	inviterId, err := GetUserIdByAffCode(user.AffCode)
	assert.Zero(t, inviterId)
	assert.Error(t, err)

	invitation, err := GetActiveUserInvitation(user.Id)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusActive, invitation.Status)
	assert.Equal(t, "launch partner", invitation.Remark)
	assert.NotEqual(t, issue.LinkToken, invitation.LinkTokenHash)
	assert.NotEqual(t, issue.ManualCode, invitation.ManualCodeHash)
	entitlements, err := invitation.GetEntitlements()
	require.NoError(t, err)
	assert.Equal(t, 12345, entitlements.Quota)
	assert.Equal(t, "vip", entitlements.Group)
	assert.Nil(t, entitlements.Plan)
}

func TestCreateUserInvitationRejectsValidityBeyondThirtyDays(t *testing.T) {
	useUserInvitationTestDB(t)

	_, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7, Group: "default", ExpiresAt: time.Now().Add(31 * 24 * time.Hour).Unix(),
	})
	assert.ErrorIs(t, err, ErrUserInvitationInvalid)

	var userCount int64
	require.NoError(t, DB.Model(&User{}).Count(&userCount).Error)
	assert.Zero(t, userCount)
}

func TestClaimUserInvitationActivatesThePrecreatedUserAtomically(t *testing.T) {
	useUserInvitationTestDB(t)

	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7,
		Quota:     8000,
		Group:     "default",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	result, err := ClaimUserInvitation(UserInvitationClaim{
		CredentialType: UserInvitationCredentialLink,
		Credential:     issue.LinkToken,
		Username:       "claimed-user",
		Password:       "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, issue.Invitation.UserId, result.User.Id)
	assert.Equal(t, common.UserStatusEnabled, result.User.Status)
	assert.Equal(t, "claimed-user", result.User.Username)
	assert.Equal(t, "claimed-user", result.User.DisplayName)
	assert.NotZero(t, result.User.ActivatedAt)
	assert.True(t, common.ValidatePasswordAndHash("password123", result.User.Password))
	inviterId, err := GetUserIdByAffCode(result.User.AffCode)
	require.NoError(t, err)
	assert.Equal(t, result.User.Id, inviterId)

	invitation, err := GetUserInvitationById(issue.Invitation.Id)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusClaimed, invitation.Status)
	assert.NotZero(t, invitation.ClaimedAt)
	assert.Equal(t, UserInvitationCredentialLink, invitation.ClaimMethod)

	_, err = ClaimUserInvitation(UserInvitationClaim{
		CredentialType: UserInvitationCredentialLink,
		Credential:     issue.LinkToken,
		Username:       "second-user",
		Password:       "password123",
	})
	assert.ErrorIs(t, err, ErrUserInvitationClaimed)
}

func TestClaimUserInvitationFailureLeavesInvitationAndUserPending(t *testing.T) {
	useUserInvitationTestDB(t)
	require.NoError(t, DB.Create(&User{
		Username: "taken-name", Password: "hash", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "taken-aff",
	}).Error)
	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7, Group: "default", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	_, err = ClaimUserInvitation(UserInvitationClaim{
		CredentialType: UserInvitationCredentialCode,
		Credential:     issue.ManualCode,
		Username:       "taken-name",
		Password:       "password123",
	})
	assert.ErrorIs(t, err, ErrUserInvitationUsernameUsed)

	invitation, err := GetUserInvitationById(issue.Invitation.Id)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusActive, invitation.Status)
	user, err := GetUserById(issue.Invitation.UserId, true)
	require.NoError(t, err)
	assert.Equal(t, common.UserStatusPendingClaim, user.Status)
	assert.Zero(t, user.ActivatedAt)
	assert.Empty(t, user.Password)
}

func TestClaimUserInvitationStartsSubscriptionFromPromisedSnapshot(t *testing.T) {
	useUserInvitationTestDB(t)
	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7,
		Quota:     500,
		Group:     "vip",
		Plan: &InvitationPlanSnapshot{
			PlanId: 23, Title: "Partner Monthly", DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
			TotalAmount: 9000, QuotaResetPeriod: SubscriptionResetWeekly,
			AllowWalletOverflow: false, UpgradeGroup: "pro", DowngradeGroup: "vip",
		},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	claimStarted := time.Now().Unix()

	result, err := ClaimUserInvitation(UserInvitationClaim{
		CredentialType: UserInvitationCredentialLink, Credential: issue.LinkToken,
		Username: "subscriber", Password: "password123",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Subscription)
	assert.GreaterOrEqual(t, result.Subscription.StartTime, claimStarted)
	assert.Equal(t, int64(9000), result.Subscription.AmountTotal)
	assert.Equal(t, "Partner Monthly", result.Subscription.PlanTitle)
	assert.Equal(t, SubscriptionResetWeekly, result.Subscription.QuotaResetPeriod)
	assert.False(t, result.Subscription.AllowWalletOverflow)
	assert.Equal(t, "vip", result.Subscription.PrevUserGroup)
	assert.Equal(t, "pro", result.User.Group)

	consumed, err := PreConsumeUserSubscription("invite-snapshot-request", result.User.Id, "model", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(10), consumed.PreConsumed)
}

func TestUpdateAndReissueUserInvitationPreserveHistoryAndReplaceCredentials(t *testing.T) {
	useUserInvitationTestDB(t)
	first, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7, Quota: 100, Group: "default", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	updated, err := UpdateUserInvitation(UserInvitationUpdate{
		UserId: first.Invitation.UserId, Quota: 250, Group: "vip", Remark: "updated",
		ExpiresAt: time.Now().Add(2 * time.Hour).Unix(),
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Remark)
	user, err := GetUserById(first.Invitation.UserId, false)
	require.NoError(t, err)
	assert.Equal(t, 250, user.Quota)
	assert.Equal(t, "vip", user.Group)

	reissued, err := ReissueUserInvitation(UserInvitationReissue{
		UserId: first.Invitation.UserId, CreatedBy: 9, Quota: 300, Group: "vip",
		ExpiresAt: time.Now().Add(3 * time.Hour).Unix(),
	})
	require.NoError(t, err)
	assert.NotEqual(t, first.LinkToken, reissued.LinkToken)
	firstRecord, err := GetUserInvitationById(first.Invitation.Id)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusRevoked, firstRecord.Status)

	_, err = GetUserInvitationPreview(UserInvitationCredentialLink, first.LinkToken)
	assert.ErrorIs(t, err, ErrUserInvitationRevoked)
	preview, err := GetUserInvitationPreview(UserInvitationCredentialLink, reissued.LinkToken)
	require.NoError(t, err)
	assert.Equal(t, reissued.Invitation.Id, preview.Invitation.Id)
	assert.Equal(t, 300, preview.Entitlements.Quota)
	assert.Equal(t, "vip", preview.Entitlements.Group)

	history, err := ListUserInvitations(first.Invitation.UserId)
	require.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestRevokeAndDeletePendingInvitedUserKeepInvitationHistory(t *testing.T) {
	useUserInvitationTestDB(t)
	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7, Group: "default", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	revoked, err := RevokeUserInvitation(issue.Invitation.UserId)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusRevoked, revoked.Status)
	_, err = GetUserInvitationPreview(UserInvitationCredentialCode, issue.ManualCode)
	assert.ErrorIs(t, err, ErrUserInvitationRevoked)

	reissued, err := ReissueUserInvitation(UserInvitationReissue{
		UserId: issue.Invitation.UserId, CreatedBy: 7, Group: "default",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	require.NoError(t, DeletePendingInvitedUser(issue.Invitation.UserId))

	var deleted User
	require.NoError(t, DB.Unscoped().First(&deleted, issue.Invitation.UserId).Error)
	assert.True(t, deleted.DeletedAt.Valid)
	latest, err := GetUserInvitationById(reissued.Invitation.Id)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusRevoked, latest.Status)
	history, err := ListUserInvitations(issue.Invitation.UserId)
	require.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestClaimUserInvitationRollsBackWhenSubscriptionCreationFails(t *testing.T) {
	useUserInvitationTestDB(t)

	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7,
		Quota:     900,
		Group:     "default",
		Plan: &InvitationPlanSnapshot{
			PlanId: 41, Title: "Invalid custom duration", DurationUnit: SubscriptionDurationCustom,
			CustomSeconds: 0, TotalAmount: 1000, QuotaResetPeriod: SubscriptionResetNever,
		},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	_, err = ClaimUserInvitation(UserInvitationClaim{
		CredentialType: UserInvitationCredentialLink, Credential: issue.LinkToken,
		Username: "rollback-user", Password: "password123",
	})
	require.Error(t, err)

	invitation, err := GetUserInvitationById(issue.Invitation.Id)
	require.NoError(t, err)
	assert.Equal(t, UserInvitationStatusActive, invitation.Status)
	user, err := GetUserById(issue.Invitation.UserId, false)
	require.NoError(t, err)
	assert.Equal(t, common.UserStatusPendingClaim, user.Status)
	assert.Empty(t, user.Password)
	assert.Zero(t, user.ActivatedAt)
	var subscriptionCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subscriptionCount).Error)
	assert.Zero(t, subscriptionCount)
}

func TestClaimUserInvitationCreatesDefaultTokenOnlyAfterActivation(t *testing.T) {
	useUserInvitationTestDB(t)
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	constant.GenerateDefaultToken = true
	t.Cleanup(func() {
		constant.GenerateDefaultToken = previousGenerateDefaultToken
	})

	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7, Group: "default", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	var tokenCount int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", issue.Invitation.UserId).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount)

	_, err = ClaimUserInvitation(UserInvitationClaim{
		CredentialType: UserInvitationCredentialCode, Credential: issue.ManualCode,
		Username: "token-user", Password: "password123",
	})
	require.NoError(t, err)
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", issue.Invitation.UserId).Count(&tokenCount).Error)
	assert.EqualValues(t, 1, tokenCount)
}

func TestInitializeUserActivatedAtBackfillsExistingUsersOnly(t *testing.T) {
	useUserInvitationTestDB(t)

	enabled := User{
		Username: "existing-enabled", Password: "hash", Status: common.UserStatusEnabled,
		Group: "default", AffCode: "existing-enabled-aff", CreatedAt: 1_700_000_001,
	}
	pending := User{
		Username: "existing-pending", Password: "", Status: common.UserStatusPendingClaim,
		Group: "default", AffCode: "existing-pending-aff", CreatedAt: 1_700_000_002,
	}
	deleted := User{
		Username: "existing-deleted", Password: "hash", Status: common.UserStatusEnabled,
		Group: "default", AffCode: "existing-deleted-aff", CreatedAt: 1_700_000_003,
	}
	require.NoError(t, DB.Create(&enabled).Error)
	require.NoError(t, DB.Create(&pending).Error)
	require.NoError(t, DB.Create(&deleted).Error)
	require.NoError(t, DB.Delete(&deleted).Error)

	require.NoError(t, InitializeUserActivatedAt())

	for _, expected := range []struct {
		id          int
		activatedAt int64
	}{
		{id: enabled.Id, activatedAt: enabled.CreatedAt},
		{id: pending.Id, activatedAt: 0},
		{id: deleted.Id, activatedAt: deleted.CreatedAt},
	} {
		var user User
		require.NoError(t, DB.Unscoped().First(&user, expected.id).Error)
		assert.Equal(t, expected.activatedAt, user.ActivatedAt)
	}
}

func TestClaimUserInvitationConcurrentClaimsHaveSingleWinner(t *testing.T) {
	useUserInvitationTestDB(t)

	issue, err := CreateUserInvitation(UserInvitationCreate{
		CreatedBy: 7, Group: "default", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	errorsByClaim := make([]error, 2)
	var wait sync.WaitGroup
	wait.Add(len(errorsByClaim))
	for index := range errorsByClaim {
		go func(index int) {
			defer wait.Done()
			_, errorsByClaim[index] = ClaimUserInvitation(UserInvitationClaim{
				CredentialType: UserInvitationCredentialLink, Credential: issue.LinkToken,
				Username: fmt.Sprintf("claim-winner-%d", index), Password: "password123",
			})
		}(index)
	}
	wait.Wait()

	successCount := 0
	claimedCount := 0
	for _, claimErr := range errorsByClaim {
		switch {
		case claimErr == nil:
			successCount++
		case errors.Is(claimErr, ErrUserInvitationClaimed):
			claimedCount++
		default:
			require.NoError(t, claimErr)
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, claimedCount)

	var activatedUsers int64
	require.NoError(t, DB.Model(&User{}).Where("id = ? AND status = ?", issue.Invitation.UserId, common.UserStatusEnabled).Count(&activatedUsers).Error)
	assert.EqualValues(t, 1, activatedUsers)
}
