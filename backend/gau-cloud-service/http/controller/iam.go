package controller

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-service/entity"
	"github.com/tnqbao/gau-cloud-service/http/controller/dto"
	"github.com/tnqbao/gau-cloud-service/infra/produce"
	"github.com/tnqbao/gau-cloud-service/utils"
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
		iamUser.ID.String(), iamUser.UserId.String(), iamUser.Name, MaskAccessKey(iamUser.AccessKey), policyName)

	// Don't return iam_policy to client - only return basic IAM user info
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

	// Parse IAM ID
	iamID, err := uuid.Parse(req.IAMID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Invalid IAM ID format: %v", err)
		utils.JSON400(c, "Invalid IAM ID format")
		return
	}

	// Get IAM user from database to verify ownership
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

	// Get policy from database
	policies, err := ctrl.Repository.IAMPolicyRepo.GetByIAMID(iamID)
	if err != nil || len(policies) == 0 {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to get policies: %v", err)
		utils.JSON500(c, "Failed to get IAM policies")
		return
	}

	oldPolicy := policies[0]
	oldPolicyName := iamUser.AccessKey + "-s3-policy"
	newPolicyName := req.AccessKey + "-s3-policy"

	// Step 1: Delete old IAM user from MinIO
	err = ctrl.Infra.Minio.DeleteIAMUser(ctx, iamUser.AccessKey)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to delete old IAM user from MinIO: %v", err)
		utils.JSON500(c, "Failed to delete old IAM user from MinIO")
		return
	}

	// Step 2: Create new IAM user with new credentials on MinIO
	err = ctrl.Infra.Minio.CreateIAMUser(ctx, req.AccessKey, req.SecretKey)
	if err != nil {
		// Rollback: Recreate old user
		_ = ctrl.Infra.Minio.CreateIAMUser(ctx, iamUser.AccessKey, iamUser.SecretKey)
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create new IAM user on MinIO: %v", err)
		utils.JSON500(c, "Failed to create new IAM user on MinIO")
		return
	}

	// Step 3: Update access_key and secret_key in database
	iamUser.AccessKey = req.AccessKey
	iamUser.SecretKey = req.SecretKey

	err = ctrl.Repository.IAMUserRepo.Update(iamUser)
	if err != nil {
		// Rollback: Delete new user and recreate old user
		_ = ctrl.Infra.Minio.DeleteIAMUser(ctx, req.AccessKey)
		_ = ctrl.Infra.Minio.CreateIAMUser(ctx, iamUser.AccessKey, iamUser.SecretKey)
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to update IAM user in database: %v", err)
		utils.JSON500(c, "Failed to update IAM user in database")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully updated IAM credentials for ID: %s", iamID.String())

	// Step 4: Async update policy on MinIO via message queue
	msg := produce.UpdateIAMPolicyMessage{
		IAMID:         req.IAMID,
		OldPolicyName: oldPolicyName,
		NewPolicyName: newPolicyName,
		PolicyJSON:    oldPolicy.Policy,
		Timestamp:     time.Now().Unix(),
	}

	err = ctrl.Infra.Produce.IAMService.PublishUpdatePolicy(ctx, msg)
	if err != nil {
		// Log error but don't fail the request since credentials already updated
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to publish update policy message (non-critical): %v", err)
	}

	utils.JSON200(c, gin.H{
		"message": "Update IAM successfully",
		"iam_id":  iamID.String(),
	})
}
