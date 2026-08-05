package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	defaultUserInvitationDays = 7
	maxUserInvitationDays     = 30
)

type adminUserInvitationRequest struct {
	Quota     int    `json:"quota"`
	Group     string `json:"group"`
	PlanId    int    `json:"plan_id"`
	Remark    string `json:"remark"`
	ExpiresAt int64  `json:"expires_at"`
}

type userInvitationClaimRequest struct {
	CredentialType   string `json:"credential_type"`
	Credential       string `json:"credential"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
}

type userInvitationAdminData struct {
	Invitation   model.UserInvitation             `json:"invitation"`
	Entitlements model.UserInvitationEntitlements `json:"entitlements"`
}

func normalizeAdminUserInvitationRequest(req *adminUserInvitationRequest, existingPlan *model.InvitationPlanSnapshot) (*model.InvitationPlanSnapshot, error) {
	if req == nil || req.Quota < 0 || int64(req.Quota) > math.MaxInt32 {
		return nil, model.ErrUserInvitationInvalid
	}
	req.Group = strings.TrimSpace(req.Group)
	if req.Group == "" {
		req.Group = "default"
	}
	if !ratio_setting.ContainsGroupRatio(req.Group) {
		return nil, errors.New("invitation group does not exist")
	}
	req.Remark = strings.TrimSpace(req.Remark)
	if len(req.Remark) > 255 {
		return nil, model.ErrUserInvitationInvalid
	}
	now := time.Now()
	if req.ExpiresAt == 0 {
		req.ExpiresAt = now.Add(defaultUserInvitationDays * 24 * time.Hour).Unix()
	}
	if req.ExpiresAt <= now.Unix() || req.ExpiresAt > now.Add(maxUserInvitationDays*24*time.Hour).Unix() {
		return nil, errors.New("invitation expiry must be within 30 days")
	}
	if req.PlanId <= 0 {
		return nil, nil
	}
	if existingPlan != nil && existingPlan.PlanId == req.PlanId {
		plan := *existingPlan
		if err := validateInvitationEntitlementGroups(&model.UserInvitationEntitlements{
			Group: req.Group, Plan: &plan,
		}); err != nil {
			return nil, err
		}
		return &plan, nil
	}
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return nil, errors.New("payment compliance confirmation is required")
	}
	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		return nil, err
	}
	if !plan.Enabled {
		return nil, errors.New("subscription plan is disabled")
	}
	for _, group := range []string{strings.TrimSpace(plan.UpgradeGroup), strings.TrimSpace(plan.DowngradeGroup)} {
		if group != "" && !ratio_setting.ContainsGroupRatio(group) {
			return nil, errors.New("subscription plan references a missing group")
		}
	}
	allowWalletOverflow := true
	if plan.AllowWalletOverflow != nil {
		allowWalletOverflow = *plan.AllowWalletOverflow
	}
	return &model.InvitationPlanSnapshot{
		PlanId:                  plan.Id,
		Title:                   plan.Title,
		DurationUnit:            plan.DurationUnit,
		DurationValue:           plan.DurationValue,
		CustomSeconds:           plan.CustomSeconds,
		TotalAmount:             plan.TotalAmount,
		QuotaResetPeriod:        model.NormalizeResetPeriod(plan.QuotaResetPeriod),
		QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
		AllowWalletOverflow:     allowWalletOverflow,
		UpgradeGroup:            strings.TrimSpace(plan.UpgradeGroup),
		DowngradeGroup:          strings.TrimSpace(plan.DowngradeGroup),
	}, nil
}

func validateInvitationEntitlementGroups(entitlements *model.UserInvitationEntitlements) error {
	if entitlements == nil || !ratio_setting.ContainsGroupRatio(entitlements.Group) {
		return errors.New("invitation group no longer exists")
	}
	if entitlements.Plan == nil {
		return nil
	}
	for _, group := range []string{entitlements.Plan.UpgradeGroup, entitlements.Plan.DowngradeGroup} {
		if group != "" && !ratio_setting.ContainsGroupRatio(group) {
			return errors.New("invitation subscription group no longer exists")
		}
	}
	return nil
}

func userInvitationAdminView(invitation model.UserInvitation) (*userInvitationAdminData, error) {
	entitlements, err := invitation.GetEntitlements()
	if err != nil {
		return nil, err
	}
	return &userInvitationAdminData{Invitation: invitation, Entitlements: *entitlements}, nil
}

func writeUserInvitationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrUserInvitationInvalid):
		common.ApiErrorMsg(c, "Invalid invitation")
	case errors.Is(err, model.ErrUserInvitationExpired):
		common.ApiErrorMsg(c, "This invitation has expired")
	case errors.Is(err, model.ErrUserInvitationClaimed):
		common.ApiErrorMsg(c, "This invitation has already been claimed")
	case errors.Is(err, model.ErrUserInvitationRevoked):
		common.ApiErrorMsg(c, "This invitation has been revoked")
	case errors.Is(err, model.ErrUserInvitationUsernameUsed):
		common.ApiErrorI18n(c, i18n.MsgUserExists)
	case errors.Is(err, model.ErrEmailAlreadyTaken):
		common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
	default:
		common.ApiError(c, err)
	}
}

func recordInvitationClaimFailure(c *gin.Context, userId int, method, reason string) {
	if method != model.UserInvitationCredentialLink && method != model.UserInvitationCredentialCode {
		method = "unknown"
	}
	params := map[string]interface{}{
		"claim_method": method,
		"reason":       reason,
		"user_agent":   c.Request.UserAgent(),
	}
	if userId > 0 {
		params["target_user_id"] = userId
	}
	recordUserSecurityAudit(c, 0, "user.invitation_claim_failed", params)
}

func invitationClaimFailureReason(err error) string {
	switch {
	case errors.Is(err, model.ErrUserInvitationExpired):
		return "expired"
	case errors.Is(err, model.ErrUserInvitationClaimed):
		return "already_claimed"
	case errors.Is(err, model.ErrUserInvitationRevoked):
		return "revoked"
	case errors.Is(err, model.ErrUserInvitationUsernameUsed):
		return "username_unavailable"
	case errors.Is(err, model.ErrEmailAlreadyTaken):
		return "email_unavailable"
	default:
		return "invalid_or_internal"
	}
}

func rejectPendingClaimUser(c *gin.Context, user *model.User) bool {
	if user == nil || user.Status != common.UserStatusPendingClaim {
		return false
	}
	common.ApiErrorMsg(c, "Pending users must be managed through their invitation")
	return true
}

func AdminCreateUserInvitation(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordLoginDisabled)
		return
	}
	var req adminUserInvitationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	plan, err := normalizeAdminUserInvitationRequest(&req, nil)
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	issue, err := model.CreateUserInvitation(model.UserInvitationCreate{
		CreatedBy: c.GetInt("id"), Quota: req.Quota, Group: req.Group, Plan: plan,
		Remark: req.Remark, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	view, err := userInvitationAdminView(issue.Invitation)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, issue.Invitation.UserId, "user.invitation_create", map[string]interface{}{
		"invitation_id": issue.Invitation.Id, "quota": req.Quota, "group": req.Group,
		"plan_id": req.PlanId, "expires_at": req.ExpiresAt,
	})
	common.ApiSuccess(c, gin.H{
		"invitation": view.Invitation, "entitlements": view.Entitlements,
		"link_token": issue.LinkToken, "manual_code": issue.ManualCode,
	})
}

func AdminListUserInvitations(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	invitations, err := model.ListUserInvitations(userId)
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	result := make([]userInvitationAdminData, 0, len(invitations))
	for _, invitation := range invitations {
		view, err := userInvitationAdminView(invitation)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		result = append(result, *view)
	}
	common.ApiSuccess(c, result)
}

func AdminUpdateUserInvitation(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req adminUserInvitationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	existingInvitation, err := model.GetActiveUserInvitation(userId)
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	existingEntitlements, err := existingInvitation.GetEntitlements()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	plan, err := normalizeAdminUserInvitationRequest(&req, existingEntitlements.Plan)
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	invitation, err := model.UpdateUserInvitation(model.UserInvitationUpdate{
		UserId: userId, Quota: req.Quota, Group: req.Group, Plan: plan,
		Remark: req.Remark, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	view, err := userInvitationAdminView(*invitation)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userId, "user.invitation_update", map[string]interface{}{
		"invitation_id": invitation.Id, "quota": req.Quota, "group": req.Group,
		"plan_id": req.PlanId, "expires_at": req.ExpiresAt,
		"previous_expires_at": existingInvitation.ExpiresAt,
	})
	common.ApiSuccess(c, view)
}

func AdminReissueUserInvitation(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordLoginDisabled)
		return
	}
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req adminUserInvitationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	plan, err := normalizeAdminUserInvitationRequest(&req, nil)
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	issue, err := model.ReissueUserInvitation(model.UserInvitationReissue{
		UserId: userId, CreatedBy: c.GetInt("id"), Quota: req.Quota, Group: req.Group,
		Plan: plan, Remark: req.Remark, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	view, err := userInvitationAdminView(issue.Invitation)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userId, "user.invitation_reissue", map[string]interface{}{
		"invitation_id": issue.Invitation.Id, "quota": req.Quota, "group": req.Group,
		"plan_id": req.PlanId, "expires_at": req.ExpiresAt,
	})
	common.ApiSuccess(c, gin.H{
		"invitation": view.Invitation, "entitlements": view.Entitlements,
		"link_token": issue.LinkToken, "manual_code": issue.ManualCode,
	})
}

func AdminRevokeUserInvitation(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	invitation, err := model.RevokeUserInvitation(userId)
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	recordManageAuditFor(c, userId, "user.invitation_revoke", map[string]interface{}{
		"invitation_id": invitation.Id,
	})
	common.ApiSuccess(c, invitation)
}

func GetUserInvitationPreview(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordLoginDisabled)
		return
	}
	preview, err := model.GetUserInvitationPreview(c.Query("credential_type"), c.Query("credential"))
	if err != nil {
		writeUserInvitationError(c, err)
		return
	}
	if err := validateInvitationEntitlementGroups(&preview.Entitlements); err != nil {
		common.ApiError(c, err)
		return
	}
	preview.Invitation.CreatedBy = 0
	preview.Invitation.Remark = ""
	common.ApiSuccess(c, preview)
}

func ClaimUserInvitation(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordLoginDisabled)
		return
	}
	var req userInvitationClaimRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		recordInvitationClaimFailure(c, 0, "unknown", "malformed_request")
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = model.NormalizeEmail(req.Email)
	candidate := model.User{Username: req.Username, Password: req.Password, Email: req.Email}
	if err := common.Validate.Struct(&candidate); err != nil || req.Username == "" || req.Password == "" {
		recordInvitationClaimFailure(c, 0, req.CredentialType, "invalid_identity")
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": "invalid username or password"})
		return
	}
	if common.EmailVerificationEnabled {
		if req.Email == "" || req.VerificationCode == "" {
			recordInvitationClaimFailure(c, 0, req.CredentialType, "email_verification_required")
			common.ApiErrorI18n(c, i18n.MsgUserEmailVerificationRequired)
			return
		}
		if !common.VerifyCodeWithKey(req.Email, req.VerificationCode, common.EmailVerificationPurpose) {
			recordInvitationClaimFailure(c, 0, req.CredentialType, "invalid_email_verification")
			common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
			return
		}
	} else {
		req.Email = ""
	}
	preview, err := model.GetUserInvitationPreview(req.CredentialType, req.Credential)
	if err != nil {
		recordInvitationClaimFailure(c, 0, req.CredentialType, invitationClaimFailureReason(err))
		writeUserInvitationError(c, err)
		return
	}
	if err := validateInvitationEntitlementGroups(&preview.Entitlements); err != nil {
		recordInvitationClaimFailure(c, preview.Invitation.UserId, req.CredentialType, "entitlement_unavailable")
		common.ApiError(c, err)
		return
	}
	result, err := model.ClaimUserInvitation(model.UserInvitationClaim{
		CredentialType: req.CredentialType, Credential: req.Credential,
		Username: req.Username, Password: req.Password, Email: req.Email,
	})
	if err != nil {
		recordInvitationClaimFailure(c, preview.Invitation.UserId, req.CredentialType, invitationClaimFailureReason(err))
		writeUserInvitationError(c, err)
		return
	}
	recordUserSecurityAudit(c, result.User.Id, "user.invitation_claim", map[string]interface{}{
		"invitation_id": result.Invitation.Id, "claim_method": result.Invitation.ClaimMethod,
		"user_agent": c.Request.UserAgent(),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"user_id": result.User.Id, "username": result.User.Username},
	})
}
