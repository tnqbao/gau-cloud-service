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

	userIDStr := c.MustGet("user_id")
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

	// Check if IAM user with the same email already exists
	existsByEmail, err := ctrl.Repository.IAMUserRepo.ExistsByEmail(req.Email)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking IAM email existence: %v", err)
		utils.JSON500(c, "Error checking IAM email existence")
		return
	}

	if existsByEmail {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] IAM with email '%s' already exists", req.Email)
		utils.JSON409(c, "IAM user with this email already exists")
		return
	}

	// Create custom policy name based on access key
	policyName := req.AccessKey + "-s3-policy"

	// Build policy JSON bytes from helper (all have Resource: [])
	policyBytes := BuildPolicyJSON(req.Role)

	// Create IAM user on MinIO with custom policy
	err = ctrl.Infra.Minio.CreateIAMUserWithCustomPolicy(ctx, req.AccessKey, req.SecretKey, policyName, policyBytes)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create IAM user on MinIO: %v", err)
		utils.JSON500(c, "Failed to create IAM user on MinIO")
		return
	}

	// Create IAM user in database
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
		// Rollback: Delete IAM user and policy from MinIO if database creation fails
		rollbackErr := ctrl.Infra.Minio.DeleteIAMUser(ctx, req.AccessKey)
		if rollbackErr != nil {
			ctrl.Infra.Logger.ErrorWithContextf(ctx, rollbackErr, "[IAM] Failed to rollback MinIO IAM user after database error: %v", rollbackErr)
		}
		rollbackErr = ctrl.Infra.Minio.DeletePolicy(ctx, policyName)
		if rollbackErr != nil {
			ctrl.Infra.Logger.ErrorWithContextf(ctx, rollbackErr, "[IAM] Failed to rollback MinIO policy after database error: %v", rollbackErr)
		}
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
		// Rollback: Delete IAM user from database and MinIO
		_ = ctrl.Repository.IAMUserRepo.Delete(iamUser.ID)
		_ = ctrl.Infra.Minio.DeleteIAMUser(ctx, req.AccessKey)
		_ = ctrl.Infra.Minio.DeletePolicy(ctx, policyName)
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create IAM policy in database: %v", err)
		utils.JSON500(c, "Failed to create IAM policy in database")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully created IAM with ID: %s, UserID: %s, Name: %s, AccessKey: %s, PolicyName: %s",
		iamUser.ID.String(), iamUser.UserId.String(), iamUser.Name, iamUser.AccessKey, policyName)

	utils.JSON200(c, gin.H{
		"iam_user":   iamUser,
		"iam_policy": iamPolicy,
	})
}

func (ctrl *Controller) ListIAMs(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received ListIAMs request")
	userIDStr := c.GetString("user_id")

	// Parse user_id from string to UUID
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

	iamUser, err := ctrl.Repository.IAMUserRepo.GetByID(iamID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to get IAM user by ID: %v", err)
		utils.JSON404(c, "IAM user not found")
		return
	}

	err = ctrl.Repository.IAMUserRepo.Delete(iamID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to delete IAM user from database: %v", err)
		utils.JSON500(c, "Failed to delete IAM user from database")
		return
	}

	err = ctrl.Infra.Minio.DeleteIAMUser(ctx, iamUser.AccessKey)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to delete IAM user from MinIO: %v", err)
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] MinIO deletion failed but DB deletion succeeded for user: %s", iamUser.AccessKey)
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
