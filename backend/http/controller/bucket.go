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

func (ctrl *Controller) DeleteBucketByID(c *gin.Context) {
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

	bucketIDStr := c.Param("id")
	if bucketIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Bucket] bucket_id not provided in path")
		utils.JSON400(c, "bucket_id is required")
		return
	}

	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Invalid bucket_id format: %v", err)
		utils.JSON400(c, "Invalid bucket id format")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Bucket] Deleting bucket with ID: %s for user_id: %s", bucketID, userID)

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to retrieve bucket: %v", err)
		utils.JSON404(c, "Bucket not found")
		return
	}

	// Check if the user owns this bucket
	if bucket.OwnerID != userID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Bucket] User %s attempted to delete bucket %s owned by %s", userID, bucketID, bucket.OwnerID)
		utils.JSON403(c, "Forbidden: you don't have permission to delete this bucket")
		return
	}

	// Delete bucket from database first
	err = ctrl.Repository.BucketRepo.Delete(bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to delete bucket from database: %v", err)
		utils.JSON500(c, "Failed to delete bucket from database")
		return
	}

	// Publish message to delete bucket from MinIO and update IAM policies
	// Consumer will handle: 1) Remove all objects, 2) Delete bucket, 3) Update policies
	err = ctrl.Infra.Produce.BucketService.PublishDeleteBucket(ctx, userID.String(), bucket.Name)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Bucket] Failed to publish delete bucket message: %v", err)
		// Don't fail the request, just log the error
		// The bucket record is deleted from DB, MinIO cleanup will happen when consumer processes the message
	} else {
		ctrl.Infra.Logger.InfoWithContextf(ctx, "[Bucket] Published delete bucket message for bucket: %s", bucket.Name)
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Bucket] Successfully deleted bucket record: %s", bucketID)
	utils.JSON200(c, gin.H{
		"message": "Bucket deletion initiated successfully",
	})
}
