package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"github.com/tnqbao/gau-cloud-orchestrator/http/controller/dto"
	"github.com/tnqbao/gau-cloud-orchestrator/utils"
)

func (ctrl *Controller) CreateIAM(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received CreateIAM request")

	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[IAM] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	var req dto.CreateIAMRequestDTO

	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to bind CreateIAM request: %v", err)
		utils.JSON400(c, "Invalid request payload")
		return
	}

	// Force role to be "user" - admin role cannot be created via API
	if req.Role == "" || req.Role == "admin" {
		req.Role = "user"
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Creating IAM for user %s with name '%s', access_key '%s'",
		userID.String(), req.Name, MaskAccessKey(req.AccessKey))

	// Check if IAM user with the same name already exists
	existsByName, err := ctrl.Repository.IAMUserRepo.CheckIAMExistsByName(req.Name)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking IAM name existence: %v", err)
		utils.JSON500(c, "Error checking IAM name existence")
		return
	}

	if existsByName {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] IAM with name '%s' already exists", req.Name)
		utils.JSON409(c, "IAM user with this name already exists")
		return
	}

	// Check if IAM user with the same access key already exists
	existsByAccessKey, err := ctrl.Repository.IAMUserRepo.ExistsByAccessKey(req.AccessKey)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking IAM access key existence: %v", err)
		utils.JSON500(c, "Error checking IAM access key existence")
		return
	}

	if existsByAccessKey {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] IAM with access key '%s' already exists", req.AccessKey)
		utils.JSON409(c, "IAM user with this access key already exists")
		return
	}

	// Build policy JSON bytes from helper
	policyBytes := BuildPolicyJSON(req.Role)

	// Create IAM user in database only (Garage doesn't support IAM)
	iamUser := &entity.IAMUser{
		ID:        uuid.New(),
		UserId:    userID,
		AccessKey: req.AccessKey,
		SecretKey: req.SecretKey,
		Name:      req.Name,
		Email:     req.Email,
		Role:      req.Role,
	}

	err = ctrl.Repository.IAMUserRepo.Create(iamUser)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create IAM user in database: %v", err)
		utils.JSON500(c, "Failed to create IAM user in database")
		return
	}

	// Save policy to database
	iamPolicy := &entity.IAMPolicy{
		ID:     uuid.New(),
		IAMID:  iamUser.ID,
		Type:   "s3",
		Policy: policyBytes,
	}

	err = ctrl.Repository.IAMPolicyRepo.Create(iamPolicy)
	if err != nil {
		// Rollback: Delete IAM user from database
		_ = ctrl.Repository.IAMUserRepo.Delete(iamUser.ID)
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create IAM policy in database: %v", err)
		utils.JSON500(c, "Failed to create IAM policy in database")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully created IAM with ID: %s, UserID: %s, Name: %s, AccessKey: %s",
		iamUser.ID.String(), iamUser.UserId.String(), iamUser.Name, MaskAccessKey(iamUser.AccessKey))

	utils.JSON200(c, gin.H{
		"message": "IAM user created successfully",
		"iam_user": gin.H{
			"id":         iamUser.ID,
			"user_id":    iamUser.UserId,
			"access_key": iamUser.AccessKey,
			"secret_key": iamUser.SecretKey,
			"name":       iamUser.Name,
			"email":      iamUser.Email,
			"role":       iamUser.Role,
		},
	})
}

func (ctrl *Controller) ListIAMs(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received ListIAMs request")
	userIDStr := c.GetString("user_id")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	iamUsers, err := ctrl.Repository.IAMUserRepo.ListByUserID(userID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to list IAM users: %v", err)
		utils.JSON500(c, "Failed to list IAM users")
		return
	}

	utils.JSON200(c, gin.H{"iam_users": iamUsers})
}

func (ctrl *Controller) DeleteIAMByID(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received DeleteIAMByID request")

	iamIDStr := c.Param("id")

	iamID, err := uuid.Parse(iamIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid IAM ID format: %v", err)
		utils.JSON400(c, "Invalid IAM ID format")
		return
	}

	_, err = ctrl.Repository.IAMUserRepo.GetByID(iamID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to get IAM user by ID: %v", err)
		utils.JSON404(c, "IAM user not found")
		return
	}

	// Delete policies first
	err = ctrl.Repository.IAMPolicyRepo.DeleteByIAMID(iamID)
	if err != nil {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] Failed to delete IAM policies: %v", err)
	}

	// Delete IAM user from database only (Garage doesn't have IAM)
	err = ctrl.Repository.IAMUserRepo.Delete(iamID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to delete IAM user from database: %v", err)
		utils.JSON500(c, "Failed to delete IAM user from database")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully deleted IAM user with ID: %s", iamID.String())
	utils.JSON200(c, gin.H{"message": "IAM user deleted successfully"})
}

func (ctrl Controller) UpdateIAMByID(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received UpdateIAMByID request")

	iamIDStr := c.Param("id")
	iamID, err := uuid.Parse(iamIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid IAM ID format: %v", err)
		utils.JSON400(c, "Invalid IAM ID format")
		return
	}

	var req dto.UpdateIAMRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to bind UpdateIAM request: %v", err)
		utils.JSON400(c, "Invalid request payload")
		return
	}

	iamUser, err := ctrl.Repository.IAMUserRepo.GetByID(iamID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to get IAM user by ID: %v", err)
		utils.JSON404(c, "IAM user not found")
		return
	}

	// Update fields if provided
	if req.Name != "" {
		iamUser.Name = req.Name
	}
	if req.Email != "" {
		iamUser.Email = req.Email
	}
	if req.Role != "" {
		iamUser.Role = req.Role
	}

	err = ctrl.Repository.IAMUserRepo.Update(iamUser)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to update IAM user in database: %v", err)
		utils.JSON500(c, "Failed to update IAM user in database")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully updated IAM user with ID: %s", iamID.String())
	utils.JSON200(c, gin.H{"iam_user": iamUser})
}

func (ctrl *Controller) UpdateIAMCredentials(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received UpdateIAMCredentials request")

	// Validate JWT and get user_id
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[IAM] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	// Bind request
	var req dto.UpdatePolicyRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to bind UpdatePolicyRequest: %v", err)
		utils.JSON400(c, "Invalid request payload")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Updating credentials for IAM ID: %s, new access_key: %s",
		req.IAMID, MaskAccessKey(req.AccessKey))

	iamID, err := uuid.Parse(req.IAMID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid IAM ID format: %v", err)
		utils.JSON400(c, "Invalid IAM ID format")
		return
	}

	iamUser, err := ctrl.Repository.IAMUserRepo.GetByID(iamID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] IAM user not found: %v", err)
		utils.JSON404(c, "IAM user not found")
		return
	}

	// Verify that this IAM belongs to the authenticated user
	if iamUser.UserId != userID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] User %s attempted to update IAM %s owned by user %s", userID.String(), iamID.String(), iamUser.UserId.String())
		utils.JSON403(c, "Forbidden: You don't have permission to update this IAM")
		return
	}

	// Check if new access key already exists (avoid conflicts)
	if req.AccessKey != iamUser.AccessKey {
		exists, err := ctrl.Repository.IAMUserRepo.ExistsByAccessKey(req.AccessKey)
		if err != nil {
			ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking access key existence: %v", err)
			utils.JSON500(c, "Error checking access key existence")
			return
		}
		if exists {
			ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] Access key '%s' already exists", req.AccessKey)
			utils.JSON409(c, "Access key already exists")
			return
		}
	}

	// Update credentials in database only (Garage doesn't have IAM)
	iamUser.AccessKey = req.AccessKey
	iamUser.SecretKey = req.SecretKey

	err = ctrl.Repository.IAMUserRepo.Update(iamUser)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to update IAM user in database: %v", err)
		utils.JSON500(c, "Failed to update IAM user in database")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully updated IAM credentials for ID: %s", iamID.String())

	utils.JSON200(c, gin.H{
		"message": "Update IAM successfully",
		"iam_id":  iamID.String(),
	})
}
