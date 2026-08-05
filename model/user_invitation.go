package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	UserInvitationStatusActive  = "active"
	UserInvitationStatusClaimed = "claimed"
	UserInvitationStatusRevoked = "revoked"
	UserInvitationStatusExpired = "expired"

	userInvitationLinkTokenBytes    = 32
	userInvitationManualCodeBytes   = 10
	userInvitationMaxValiditySecond = 30 * 24 * 60 * 60
)

var (
	ErrUserInvitationInvalid      = errors.New("user invitation is invalid")
	ErrUserInvitationExpired      = errors.New("user invitation has expired")
	ErrUserInvitationClaimed      = errors.New("user invitation has already been claimed")
	ErrUserInvitationRevoked      = errors.New("user invitation has been revoked")
	ErrUserInvitationNotPending   = errors.New("user is not pending claim")
	ErrUserInvitationUsernameUsed = errors.New("username is already taken")
)

type InvitationPlanSnapshot struct {
	PlanId                  int    `json:"plan_id"`
	Title                   string `json:"title"`
	DurationUnit            string `json:"duration_unit"`
	DurationValue           int    `json:"duration_value"`
	CustomSeconds           int64  `json:"custom_seconds"`
	TotalAmount             int64  `json:"total_amount"`
	QuotaResetPeriod        string `json:"quota_reset_period"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds"`
	AllowWalletOverflow     bool   `json:"allow_wallet_overflow"`
	UpgradeGroup            string `json:"upgrade_group"`
	DowngradeGroup          string `json:"downgrade_group"`
}

type UserInvitationEntitlements struct {
	Quota int                     `json:"quota"`
	Group string                  `json:"group"`
	Plan  *InvitationPlanSnapshot `json:"plan,omitempty"`
}

type UserInvitation struct {
	Id               int            `json:"id"`
	UserId           int            `json:"user_id" gorm:"not null;index"`
	CreatedBy        int            `json:"created_by" gorm:"not null;index"`
	Status           string         `json:"status" gorm:"type:varchar(16);not null;index"`
	LinkTokenHash    string         `json:"-" gorm:"type:char(64);not null;uniqueIndex"`
	ManualCodeHash   string         `json:"-" gorm:"type:char(64);not null;uniqueIndex"`
	EntitlementsJson string         `json:"-" gorm:"type:text;not null;column:entitlements"`
	Remark           string         `json:"remark,omitempty" gorm:"type:varchar(255)"`
	ExpiresAt        int64          `json:"expires_at" gorm:"type:bigint;not null;index"`
	ClaimedAt        int64          `json:"claimed_at" gorm:"type:bigint;default:0"`
	RevokedAt        int64          `json:"revoked_at" gorm:"type:bigint;default:0"`
	ClaimMethod      string         `json:"claim_method,omitempty" gorm:"type:varchar(16)"`
	CreatedAt        int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (UserInvitation) TableName() string {
	return "user_invitations"
}

func (invitation *UserInvitation) GetEntitlements() (*UserInvitationEntitlements, error) {
	if invitation == nil || invitation.EntitlementsJson == "" {
		return nil, ErrUserInvitationInvalid
	}
	var entitlements UserInvitationEntitlements
	if err := common.UnmarshalJsonStr(invitation.EntitlementsJson, &entitlements); err != nil {
		return nil, err
	}
	return &entitlements, nil
}

type UserInvitationCreate struct {
	CreatedBy int
	Quota     int
	Group     string
	Plan      *InvitationPlanSnapshot
	Remark    string
	ExpiresAt int64
}

type UserInvitationIssue struct {
	Invitation UserInvitation `json:"invitation"`
	LinkToken  string         `json:"link_token"`
	ManualCode string         `json:"manual_code"`
}

func userInvitationHash(kind, credential string) string {
	key := []byte("user-invitation-v1:" + common.SessionSecret)
	return common.GenerateHMACWithKey(key, kind+":"+credential)
}

func normalizeUserInvitationManualCode(code string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
}

func generateUserInvitationCredentials() (string, string, error) {
	linkBytes := make([]byte, userInvitationLinkTokenBytes)
	if _, err := rand.Read(linkBytes); err != nil {
		return "", "", fmt.Errorf("generate invitation link token: %w", err)
	}
	codeBytes := make([]byte, userInvitationManualCodeBytes)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", "", fmt.Errorf("generate invitation manual code: %w", err)
	}
	linkToken := base64.RawURLEncoding.EncodeToString(linkBytes)
	code := strings.ToUpper(hex.EncodeToString(codeBytes))
	parts := make([]string, 0, len(code)/4)
	for index := 0; index < len(code); index += 4 {
		parts = append(parts, code[index:index+4])
	}
	return linkToken, strings.Join(parts, "-"), nil
}

func CreateUserInvitation(input UserInvitationCreate) (*UserInvitationIssue, error) {
	input.Group = strings.TrimSpace(input.Group)
	input.Remark = strings.TrimSpace(input.Remark)
	now := common.GetTimestamp()
	if input.CreatedBy <= 0 || input.Quota < 0 || input.Group == "" ||
		input.ExpiresAt <= now || input.ExpiresAt > now+userInvitationMaxValiditySecond {
		return nil, ErrUserInvitationInvalid
	}
	linkToken, manualCode, err := generateUserInvitationCredentials()
	if err != nil {
		return nil, err
	}
	entitlementsBytes, err := common.Marshal(UserInvitationEntitlements{
		Quota: input.Quota,
		Group: input.Group,
		Plan:  input.Plan,
	})
	if err != nil {
		return nil, err
	}
	identityBytes := make([]byte, 8)
	if _, err := rand.Read(identityBytes); err != nil {
		return nil, fmt.Errorf("generate pending user identity: %w", err)
	}
	identity := hex.EncodeToString(identityBytes)
	user := User{
		Username:    "inv_" + identity,
		DisplayName: "",
		Password:    "",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusPendingClaim,
		Quota:       input.Quota,
		Group:       input.Group,
		AffCode:     "inv" + identity,
		AuthVersion: 1,
		ActivatedAt: 0,
	}
	invitation := UserInvitation{
		CreatedBy:        input.CreatedBy,
		Status:           UserInvitationStatusActive,
		LinkTokenHash:    userInvitationHash("link", linkToken),
		ManualCodeHash:   userInvitationHash("code", normalizeUserInvitationManualCode(manualCode)),
		EntitlementsJson: string(entitlementsBytes),
		Remark:           input.Remark,
		ExpiresAt:        input.ExpiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		invitation.UserId = user.Id
		return tx.Create(&invitation).Error
	}); err != nil {
		return nil, err
	}
	return &UserInvitationIssue{
		Invitation: invitation,
		LinkToken:  linkToken,
		ManualCode: manualCode,
	}, nil
}

func GetActiveUserInvitation(userId int) (*UserInvitation, error) {
	if userId <= 0 {
		return nil, ErrUserInvitationInvalid
	}
	var invitation UserInvitation
	if err := DB.Where("user_id = ? AND status = ?", userId, UserInvitationStatusActive).
		Order("id desc").First(&invitation).Error; err != nil {
		return nil, err
	}
	return &invitation, nil
}

const (
	UserInvitationCredentialLink = "link"
	UserInvitationCredentialCode = "code"
)

type UserInvitationClaim struct {
	CredentialType string
	Credential     string
	Username       string
	Password       string
	Email          string
}

type UserInvitationClaimResult struct {
	User         User              `json:"user"`
	Invitation   UserInvitation    `json:"invitation"`
	Subscription *UserSubscription `json:"subscription,omitempty"`
}

func applyUserInvitationCredential(query *gorm.DB, credentialType, credential string) (*gorm.DB, error) {
	credential = strings.TrimSpace(credential)
	switch credentialType {
	case UserInvitationCredentialLink:
		decoded, err := base64.RawURLEncoding.DecodeString(credential)
		if err != nil || len(decoded) != userInvitationLinkTokenBytes {
			return nil, ErrUserInvitationInvalid
		}
		return query.Where("link_token_hash = ?", userInvitationHash("link", credential)), nil
	case UserInvitationCredentialCode:
		credential = normalizeUserInvitationManualCode(credential)
		decoded, err := hex.DecodeString(credential)
		if err != nil || len(decoded) != userInvitationManualCodeBytes {
			return nil, ErrUserInvitationInvalid
		}
		return query.Where("manual_code_hash = ?", userInvitationHash("code", credential)), nil
	default:
		return nil, ErrUserInvitationInvalid
	}
}

func GetUserInvitationById(id int) (*UserInvitation, error) {
	if id <= 0 {
		return nil, ErrUserInvitationInvalid
	}
	var invitation UserInvitation
	if err := DB.First(&invitation, id).Error; err != nil {
		return nil, err
	}
	return &invitation, nil
}
