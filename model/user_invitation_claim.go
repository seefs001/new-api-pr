package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

var sqliteUserInvitationWriteMutex sync.Mutex

func lockUserInvitationWriteForSQLite() bool {
	if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return false
	}
	sqliteUserInvitationWriteMutex.Lock()
	return true
}
func ClaimUserInvitation(input UserInvitationClaim) (*UserInvitationClaimResult, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = NormalizeEmail(input.Email)
	if input.Username == "" || len(input.Username) > UserNameMaxLength || len(input.Password) < 8 || len(input.Password) > 20 {
		return nil, ErrUserInvitationInvalid
	}
	passwordHash, err := common.Password2Hash(input.Password)
	if err != nil {
		return nil, err
	}
	defaultTokenKey := ""
	if constant.GenerateDefaultToken {
		defaultTokenKey, err = common.GenerateKey()
		if err != nil {
			return nil, err
		}
	}

	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}

	var result UserInvitationClaimResult
	var terminalErr error
	err = DB.Transaction(func(tx *gorm.DB) error {
		query, err := applyUserInvitationCredential(tx, input.CredentialType, input.Credential)
		if err != nil {
			return err
		}
		var invitation UserInvitation
		if err := query.First(&invitation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserInvitationInvalid
			}
			return err
		}
		var user User
		if err := lockForUpdate(tx).Where("id = ?", invitation.UserId).First(&user).Error; err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("id = ?", invitation.Id).First(&invitation).Error; err != nil {
			return err
		}
		switch invitation.Status {
		case UserInvitationStatusClaimed:
			return ErrUserInvitationClaimed
		case UserInvitationStatusRevoked:
			return ErrUserInvitationRevoked
		case UserInvitationStatusExpired:
			return ErrUserInvitationExpired
		case UserInvitationStatusActive:
		default:
			return ErrUserInvitationInvalid
		}
		now := common.GetTimestamp()
		if invitation.ExpiresAt <= now {
			terminalErr = ErrUserInvitationExpired
			return tx.Model(&invitation).Updates(map[string]interface{}{
				"status":     UserInvitationStatusExpired,
				"updated_at": now,
			}).Error
		}
		if user.Status != common.UserStatusPendingClaim || user.ActivatedAt != 0 {
			return ErrUserInvitationNotPending
		}
		var usernameCount int64
		if err := tx.Unscoped().Model(&User{}).Where("username = ? AND id <> ?", input.Username, user.Id).Count(&usernameCount).Error; err != nil {
			return err
		}
		if usernameCount > 0 {
			return ErrUserInvitationUsernameUsed
		}
		return withNormalizedEmailLock(tx, input.Email, func(tx *gorm.DB) error {
			if err := ensureEmailAvailableWithTx(tx, input.Email, user.Id); err != nil {
				return err
			}
			entitlements, err := invitation.GetEntitlements()
			if err != nil {
				return err
			}
			claim := tx.Model(&UserInvitation{}).
				Where("id = ? AND status = ? AND expires_at > ?", invitation.Id, UserInvitationStatusActive, now).
				Updates(map[string]interface{}{
					"status": UserInvitationStatusClaimed, "claimed_at": now,
					"claim_method": input.CredentialType, "updated_at": now,
				})
			if claim.Error != nil {
				return claim.Error
			}
			if claim.RowsAffected != 1 {
				return ErrUserInvitationClaimed
			}
			user.Username = input.Username
			user.DisplayName = input.Username
			user.Password = passwordHash
			user.Email = input.Email
			user.Status = common.UserStatusEnabled
			user.ActivatedAt = now
			user.Quota = entitlements.Quota
			user.Group = entitlements.Group
			settings := user.GetSetting()
			settings.SidebarModules = generateDefaultSidebarConfigForRole(common.RoleCommonUser)
			user.SetSetting(settings)
			user.AuthVersion, err = IncrementUserAuthVersionWithTx(tx, user.Id)
			if err != nil {
				return err
			}
			if err := tx.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
				"username":     user.Username,
				"display_name": user.DisplayName,
				"password":     user.Password,
				"email":        user.Email,
				"status":       user.Status,
				"activated_at": user.ActivatedAt,
				"quota":        user.Quota,
				"group":        user.Group,
				"setting":      user.Setting,
			}).Error; err != nil {
				return err
			}
			var subscription *UserSubscription
			if entitlements.Plan != nil {
				plan := SubscriptionPlan{
					Id:                      entitlements.Plan.PlanId,
					Title:                   entitlements.Plan.Title,
					DurationUnit:            entitlements.Plan.DurationUnit,
					DurationValue:           entitlements.Plan.DurationValue,
					CustomSeconds:           entitlements.Plan.CustomSeconds,
					TotalAmount:             entitlements.Plan.TotalAmount,
					QuotaResetPeriod:        entitlements.Plan.QuotaResetPeriod,
					QuotaResetCustomSeconds: entitlements.Plan.QuotaResetCustomSeconds,
					UpgradeGroup:            entitlements.Plan.UpgradeGroup,
					DowngradeGroup:          entitlements.Plan.DowngradeGroup,
					AllowWalletOverflow:     common.GetPointer(entitlements.Plan.AllowWalletOverflow),
				}
				subscription, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, &plan, "invitation")
				if err != nil {
					return err
				}
				if subscription.PrevUserGroup != "" {
					user.Group = subscription.UpgradeGroup
				}
			}
			if defaultTokenKey != "" {
				token := Token{
					UserId:             user.Id,
					Name:               user.Username + "的初始令牌",
					Key:                defaultTokenKey,
					Status:             common.TokenStatusEnabled,
					CreatedTime:        now,
					AccessedTime:       now,
					ExpiredTime:        -1,
					RemainQuota:        500000,
					UnlimitedQuota:     true,
					ModelLimitsEnabled: false,
				}
				if err := tx.Create(&token).Error; err != nil {
					return err
				}
			}
			invitation.Status = UserInvitationStatusClaimed
			invitation.ClaimedAt = now
			invitation.ClaimMethod = input.CredentialType
			result.User = user
			result.Invitation = invitation
			result.Subscription = subscription
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if terminalErr != nil {
		return nil, terminalErr
	}
	if err := PublishUserAuthCache(result.User.Id); err != nil {
		common.SysError(fmt.Sprintf("failed to publish invited user auth cache for user %d: %v", result.User.Id, err))
	}
	if result.Subscription != nil && result.Subscription.PrevUserGroup != "" {
		refreshSubscriptionUserGroupCache(result.User.Id, "invitation claim")
	}
	return &result, nil
}
