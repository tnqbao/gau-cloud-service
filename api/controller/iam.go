package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/tnqbao/gau-cloud-orchestrator/controller/dto"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
)

func (ctrl *Controller) CreateIAM(c *gin.Context) {
	ctx := c.Request.Context()
	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Received CreateIAM request")

	var req dto.CreateIAMRequestDTO

	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to bind CreateIAM request: %v", err)
		c.JSON(400, gin.H{"error": "Invalid request payload"})
		return
	}

	// Check if IAM user with the same name already exists
	existsByName, err := ctrl.Repository.IAMUserRepo.CheckIAMExistsByName(req.Name)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking IAM name existence: %v", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}

	if existsByName {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] IAM with name '%s' already exists", req.Name)
		c.JSON(400, gin.H{"error": "IAM user with this name already exists"})
		return
	}

	// Check if IAM user with the same access key already exists
	existsByAccessKey, err := ctrl.Repository.IAMUserRepo.ExistsByAccessKey(req.AccessKey)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking IAM access key existence: %v", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}

	if existsByAccessKey {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] IAM with access key '%s' already exists", req.AccessKey)
		c.JSON(400, gin.H{"error": "IAM user with this access key already exists"})
		return
	}

	// Check if IAM user with the same email already exists
	existsByEmail, err := ctrl.Repository.IAMUserRepo.ExistsByEmail(req.Email)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Error checking IAM email existence: %v", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}

	if existsByEmail {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[IAM] IAM with email '%s' already exists", req.Email)
		c.JSON(400, gin.H{"error": "IAM user with this email already exists"})
		return
	}

	// Create IAM user on MinIO first
	err = ctrl.Infra.Minio.CreateIAMUser(ctx, req.AccessKey, req.SecretKey)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create IAM user on MinIO: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create IAM user on MinIO"})
		return
	}

	// Create IAM user in database
	iamUser := &entity.IAMUser{
		AccessKey: req.AccessKey,
		SecretKey: req.SecretKey,
		Name:      req.Name,
		Email:     req.Email,
		Role:      req.Role,
	}

	err = ctrl.Repository.IAMUserRepo.Create(iamUser)
	if err != nil {
		// Rollback: Delete IAM user from MinIO if database creation fails
		rollbackErr := ctrl.Infra.Minio.DeleteIAMUser(ctx, req.AccessKey)
		if rollbackErr != nil {
			ctrl.Infra.Logger.ErrorWithContextf(ctx, rollbackErr, "[IAM] Failed to rollback MinIO IAM user after database error: %v", rollbackErr)
		}
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[IAM] Failed to create IAM user in database: %v", err)
		c.JSON(500, gin.H{"error": "Failed to create IAM user"})
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[IAM] Successfully created IAM with ID: %d, Name: %s, AccessKey: %s", iamUser.ID, iamUser.Name, iamUser.AccessKey)

	c.JSON(200, gin.H{
		"message": "Create IAM success",
		"data": gin.H{
			"id":         iamUser.ID,
			"name":       iamUser.Name,
			"email":      iamUser.Email,
			"access_key": iamUser.AccessKey,
			"role":       iamUser.Role,
		},
	})
}
