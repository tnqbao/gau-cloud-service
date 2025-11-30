package controller

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"github.com/tnqbao/gau-cloud-orchestrator/http/controller/dto"
	"github.com/tnqbao/gau-cloud-orchestrator/utils"
)

func (ctrl *Controller) CreateBucket(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Bucket] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	var req dto.CreateBucketRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to bind JSON: %v", err)
		utils.JSON400(c, "Invalid request payload")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Bucket] Creating bucket '%s' in region '%s' for user_id: %s",
		req.Name, req.Region, userID)

	// Check if bucket with the same name already exists
	existsByName, err := ctrl.Repository.BucketRepo.ExistsByName(req.Name)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Error checking bucket name existence: %v", err)
		utils.JSON500(c, "Error checking bucket name existence")
		return
	}

	if existsByName {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Bucket] Bucket with name '%s' already exists", req.Name)
		utils.JSON409(c, "Bucket with this name already exists")
		return
	}

	// Create bucket on MinIO
	err = ctrl.Infra.Minio.CreateBucket(ctx, req.Name, req.Region)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to create bucket on MinIO: %v", err)
		utils.JSON500(c, "Failed to create bucket on MinIO")
		return
	}

	bucket := &entity.Bucket{
		ID:        uuid.New(),
		Name:      req.Name,
		Region:    req.Region,
		CreatedAt: time.Now().Format(time.RFC3339),
		OwnerID:   userID,
	}

	err = ctrl.Repository.BucketRepo.Create(bucket)
	if err != nil {
		// Rollback
		rollbackErr := ctrl.Infra.Minio.DeleteBucket(ctx, req.Name)
		if rollbackErr != nil {
			ctrl.Infra.Logger.ErrorWithContextf(ctx, rollbackErr, "[Bucket] Failed to rollback MinIO bucket after database error: %v", rollbackErr)
		}
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to create bucket in database: %v", err)
		utils.JSON500(c, "Failed to create bucket in database")
		return
	}

	// Publish message to update IAM policies for all user's IAM users
	err = ctrl.Infra.Produce.BucketService.PublishUpdateBucketPolicy(ctx, userID.String(), req.Name)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to publish update policy message: %v", err)
		// Don't fail the request, just log the error
		// The bucket is already created, policy update can be done manually if needed
	} else {
		ctrl.Infra.Logger.InfoWithContextf(ctx, "[Bucket] Published update policy message for bucket: %s", req.Name)
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Bucket] Successfully created bucket: %s", bucket.ID)
	utils.JSON200(c, gin.H{
		"message": "Bucket created successfully",
		"bucket":  bucket,
	})
}
