package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type UserInvitationUpdate struct {
	UserId    int
	Quota     int
	Group     string
	Plan      *InvitationPlanSnapshot
	Remark    string
	ExpiresAt int64
}

type UserInvitationReissue struct {
	UserId    int
	CreatedBy int
	Quota     int
	Group     string
	Plan      *InvitationPlanSnapshot
	Remark    string
	ExpiresAt int64
}

type UserInvitationPreview struct {
	Invitation   UserInvitation             `json:"invitation"`
	Entitlements UserInvitationEntitlements `json:"entitlements"`
}

func marshalUserInvitationEntitlements(quota int, group string, plan *InvitationPlanSnapshot) (string, error) {
	bytes, err := common.Marshal(UserInvitationEntitlements{Quota: quota, Group: group, Plan: plan})
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func validateUserInvitationBenefits(userId, quota int, group string, expiresAt int64) error {
	now := common.GetTimestamp()
	if userId <= 0 || quota < 0 || strings.TrimSpace(group) == "" ||
		expiresAt <= now || expiresAt > now+userInvitationMaxValiditySecond {
		return ErrUserInvitationInvalid
	}
	return nil
}

func UpdateUserInvitation(input UserInvitationUpdate) (*UserInvitation, error) {
	input.Group = strings.TrimSpace(input.Group)
	input.Remark = strings.TrimSpace(input.Remark)
	if err := validateUserInvitationBenefits(input.UserId, input.Quota, input.Group, input.ExpiresAt); err != nil {
		return nil, err
	}
	entitlements, err := marshalUserInvitationEntitlements(input.Quota, input.Group, input.Plan)
	if err != nil {
		return nil, err
	}
	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}

	var updated UserInvitation
	var terminalErr error
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", input.UserId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusPendingClaim {
			return ErrUserInvitationNotPending
		}
		if err := lockForUpdate(tx).Where("user_id = ? AND status = ?", input.UserId, UserInvitationStatusActive).
			Order("id desc").First(&updated).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if updated.ExpiresAt <= now {
			terminalErr = ErrUserInvitationExpired
			return tx.Model(&updated).Updates(map[string]interface{}{
				"status": UserInvitationStatusExpired, "updated_at": now,
			}).Error
		}
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"quota": input.Quota, "group": input.Group,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&updated).Updates(map[string]interface{}{
			"entitlements": entitlements, "remark": input.Remark,
			"expires_at": input.ExpiresAt, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		updated.EntitlementsJson = entitlements
		updated.Remark = input.Remark
		updated.ExpiresAt = input.ExpiresAt
		updated.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	if terminalErr != nil {
		return nil, terminalErr
	}
	_ = InvalidateUserCache(input.UserId)
	return &updated, nil
}

func ReissueUserInvitation(input UserInvitationReissue) (*UserInvitationIssue, error) {
	input.Group = strings.TrimSpace(input.Group)
	input.Remark = strings.TrimSpace(input.Remark)
	if input.CreatedBy <= 0 {
		return nil, ErrUserInvitationInvalid
	}
	if err := validateUserInvitationBenefits(input.UserId, input.Quota, input.Group, input.ExpiresAt); err != nil {
		return nil, err
	}
	entitlements, err := marshalUserInvitationEntitlements(input.Quota, input.Group, input.Plan)
	if err != nil {
		return nil, err
	}
	linkToken, manualCode, err := generateUserInvitationCredentials()
	if err != nil {
		return nil, err
	}
	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}

	now := common.GetTimestamp()
	invitation := UserInvitation{
		UserId: input.UserId, CreatedBy: input.CreatedBy, Status: UserInvitationStatusActive,
		LinkTokenHash:    userInvitationHash("link", linkToken),
		ManualCodeHash:   userInvitationHash("code", normalizeUserInvitationManualCode(manualCode)),
		EntitlementsJson: entitlements, Remark: input.Remark, ExpiresAt: input.ExpiresAt,
		CreatedAt: now, UpdatedAt: now,
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", input.UserId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusPendingClaim {
			return ErrUserInvitationNotPending
		}
		var active []UserInvitation
		if err := lockForUpdate(tx).Where("user_id = ? AND status = ?", input.UserId, UserInvitationStatusActive).Find(&active).Error; err != nil {
			return err
		}
		for index := range active {
			status := UserInvitationStatusRevoked
			updates := map[string]interface{}{"status": status, "revoked_at": now, "updated_at": now}
			if active[index].ExpiresAt <= now {
				updates["status"] = UserInvitationStatusExpired
				updates["revoked_at"] = 0
			}
			if err := tx.Model(&active[index]).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"quota": input.Quota, "group": input.Group,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&invitation).Error
	})
	if err != nil {
		return nil, err
	}
	_ = InvalidateUserCache(input.UserId)
	return &UserInvitationIssue{Invitation: invitation, LinkToken: linkToken, ManualCode: manualCode}, nil
}

func GetUserInvitationPreview(credentialType, credential string) (*UserInvitationPreview, error) {
	query, err := applyUserInvitationCredential(DB, credentialType, credential)
	if err != nil {
		return nil, err
	}
	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}
	var invitation UserInvitation
	if err := query.First(&invitation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserInvitationInvalid
		}
		return nil, err
	}
	if invitation.Status == UserInvitationStatusActive && invitation.ExpiresAt <= common.GetTimestamp() {
		now := common.GetTimestamp()
		if err := DB.Model(&invitation).Where("status = ?", UserInvitationStatusActive).
			Updates(map[string]interface{}{"status": UserInvitationStatusExpired, "updated_at": now}).Error; err != nil {
			return nil, err
		}
		invitation.Status = UserInvitationStatusExpired
	}
	switch invitation.Status {
	case UserInvitationStatusClaimed:
		return nil, ErrUserInvitationClaimed
	case UserInvitationStatusRevoked:
		return nil, ErrUserInvitationRevoked
	case UserInvitationStatusExpired:
		return nil, ErrUserInvitationExpired
	case UserInvitationStatusActive:
	default:
		return nil, ErrUserInvitationInvalid
	}
	entitlements, err := invitation.GetEntitlements()
	if err != nil {
		return nil, err
	}
	return &UserInvitationPreview{Invitation: invitation, Entitlements: *entitlements}, nil
}

func ListUserInvitations(userId int) ([]UserInvitation, error) {
	if userId <= 0 {
		return nil, ErrUserInvitationInvalid
	}
	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}

	now := common.GetTimestamp()
	if err := DB.Model(&UserInvitation{}).Where("user_id = ? AND status = ? AND expires_at <= ?", userId, UserInvitationStatusActive, now).
		Updates(map[string]interface{}{"status": UserInvitationStatusExpired, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	var invitations []UserInvitation
	if err := DB.Where("user_id = ?", userId).Order("id desc").Find(&invitations).Error; err != nil {
		return nil, err
	}
	return invitations, nil
}

func RevokeUserInvitation(userId int) (*UserInvitation, error) {
	if userId <= 0 {
		return nil, ErrUserInvitationInvalid
	}
	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}

	var invitation UserInvitation
	var terminalErr error
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusPendingClaim {
			return ErrUserInvitationNotPending
		}
		if err := lockForUpdate(tx).Where("user_id = ? AND status = ?", userId, UserInvitationStatusActive).
			Order("id desc").First(&invitation).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		if invitation.ExpiresAt <= now {
			terminalErr = ErrUserInvitationExpired
			invitation.Status = UserInvitationStatusExpired
			return tx.Model(&invitation).Updates(map[string]interface{}{
				"status": UserInvitationStatusExpired, "updated_at": now,
			}).Error
		}
		invitation.Status = UserInvitationStatusRevoked
		invitation.RevokedAt = now
		invitation.UpdatedAt = now
		return tx.Model(&invitation).Updates(map[string]interface{}{
			"status": UserInvitationStatusRevoked, "revoked_at": now, "updated_at": now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	if terminalErr != nil {
		return nil, terminalErr
	}
	return &invitation, nil
}

func DeletePendingInvitedUser(userId int) error {
	if userId <= 0 {
		return ErrUserInvitationInvalid
	}
	if lockUserInvitationWriteForSQLite() {
		defer sqliteUserInvitationWriteMutex.Unlock()
	}

	var nextAuthVersion int64
	now := common.GetTimestamp()
	if err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusPendingClaim {
			return ErrUserInvitationNotPending
		}
		if err := tx.Model(&UserInvitation{}).Where("user_id = ? AND status = ?", userId, UserInvitationStatusActive).
			Updates(map[string]interface{}{
				"status": UserInvitationStatusRevoked, "revoked_at": now, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		var err error
		nextAuthVersion, err = IncrementUserAuthVersionWithTx(tx, userId)
		if err != nil {
			return err
		}
		return tx.Delete(&user).Error
	}); err != nil {
		return err
	}
	if err := publishCommittedUserAuthVersion(userId, nextAuthVersion); err != nil {
		common.SysError(fmt.Sprintf("failed to publish pending user deletion for user %d: %v", userId, err))
	}
	return InvalidateUserCache(userId)
}

func InitializeUserActivatedAt() error {
	return DB.Unscoped().Model(&User{}).
		Where("(activated_at IS NULL OR activated_at = ?) AND status <> ?", 0, common.UserStatusPendingClaim).
		Update("activated_at", gorm.Expr("created_at")).Error
}
