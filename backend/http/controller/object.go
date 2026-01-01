package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/utils"
)

func (ctrl *Controller) UploadObject(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Object] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	// Get bucket_id from path parameter
	bucketIDStr := c.Param("id")
	if bucketIDStr == "" {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] bucket_id not provided in path")
		utils.JSON400(c, "bucket_id is required")
		return
	}

	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid bucket_id format: %v", err)
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	// Check if bucket exists and user has permission
	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Bucket not found: %v", err)
		utils.JSON404(c, "Bucket not found")
		return
	}

	// Check if the user owns this bucket
	if bucket.OwnerID != userID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] User %s attempted to upload to bucket %s owned by %s", userID, bucketID, bucket.OwnerID)
		utils.JSON403(c, "Forbidden: you don't have permission to upload to this bucket")
		return
	}

	// Get file from multipart form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to get file from form data")
		utils.JSON400(c, "Failed to get file: "+err.Error())
		return
	}

	// Optional: Get custom file path/folder (supports nested paths like abc/def)
	customPath := strings.TrimSpace(c.PostForm("path"))
	if customPath != "" {
		// Clean and normalize path: remove leading/trailing slashes, replace backslashes
		customPath = strings.Trim(customPath, "/\\")
		customPath = strings.ReplaceAll(customPath, "\\", "/")

		// Remove any double slashes
		for strings.Contains(customPath, "//") {
			customPath = strings.ReplaceAll(customPath, "//", "/")
		}

		// Validate path doesn't contain dangerous characters
		if strings.Contains(customPath, "..") {
			ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] Invalid path contains ..")
			utils.JSON400(c, "Invalid path: path cannot contain '..'")
			return
		}
	}

	// Get content type from file header
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Check file size against threshold
	largeFileThreshold := ctrl.Config.EnvConfig.LargeFile.Threshold
	if largeFileThreshold == 0 {
		largeFileThreshold = 52428800 // Default 50MB
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Uploading file '%s' (size: %d bytes) to bucket '%s' at path '%s'",
		fileHeader.Filename, fileHeader.Size, bucket.Name, customPath)

	// Route based on file size
	if fileHeader.Size > largeFileThreshold {
		// Large file flow: upload to temp MinIO and publish message for chunking
		ctrl.handleLargeFileUpload(c, fileHeader, bucket, bucketID, userID, customPath, contentType)
	} else {
		// Small file flow: use existing direct upload
		ctrl.handleSmallFileUpload(c, fileHeader, bucket, bucketID, customPath, contentType)
	}
}

func (ctrl *Controller) ListObjectsByPath(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Object] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	// Get bucket_id from path parameter
	bucketIDStr := c.Param("id")
	if bucketIDStr == "" {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] bucket_id not provided in path")
		utils.JSON400(c, "bucket_id is required")
		return
	}

	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid bucket_id format: %v", err)
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	// Check if bucket exists and user has permission
	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Bucket not found: %v", err)
		utils.JSON404(c, "Bucket not found")
		return
	}

	// Check if the user owns this bucket
	if bucket.OwnerID != userID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] User %s attempted to list objects in bucket %s owned by %s", userID, bucketID, bucket.OwnerID)
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	// Get path from wildcard parameter and normalize it
	parentPath := c.Param("path")
	// Remove leading slash from wildcard path
	parentPath = strings.TrimPrefix(parentPath, "/")
	// Remove trailing slash
	parentPath = strings.TrimSuffix(parentPath, "/")
	// Clean the path
	parentPath = strings.TrimSpace(parentPath)

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Listing objects in bucket '%s' at path '%s'", bucket.Name, parentPath)

	// Get objects at this path
	objects, err := ctrl.Repository.ObjectRepo.FindByBucketIDAndPath(bucketID, parentPath)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to list objects: %v", err)
		utils.JSON500(c, "Failed to list objects")
		return
	}

	// Get folders at this path
	folders, err := ctrl.Repository.ObjectRepo.FindFoldersByBucketIDAndPath(bucketID, parentPath)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to list folders: %v", err)
		utils.JSON500(c, "Failed to list folders")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Found %d objects and %d folders in bucket '%s' at path '%s'", len(objects), len(folders), bucket.Name, parentPath)
	utils.JSON200(c, gin.H{
		"path":         parentPath,
		"objects":      objects,
		"folders":      folders,
		"object_count": len(objects),
		"folder_count": len(folders),
	})
}

func (ctrl *Controller) DeleteObject(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Object] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid user_id format: %v", err)
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	// Get bucket_id from path parameter
	bucketIDStr := c.Param("id")
	if bucketIDStr == "" {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] bucket_id not provided in path")
		utils.JSON400(c, "bucket_id is required")
		return
	}

	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid bucket_id format: %v", err)
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	// Get object_id from path parameter
	objectIDStr := c.Param("object_id")
	if objectIDStr == "" {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] object_id not provided in path")
		utils.JSON400(c, "object_id is required")
		return
	}

	objectID, err := uuid.Parse(objectIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid object_id format: %v", err)
		utils.JSON400(c, "Invalid object_id format")
		return
	}

	// Check if bucket exists and user has permission
	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Bucket not found: %v", err)
		utils.JSON404(c, "Bucket not found")
		return
	}

	// Check if the user owns this bucket
	if bucket.OwnerID != userID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] User %s attempted to delete object in bucket %s owned by %s", userID, bucketID, bucket.OwnerID)
		utils.JSON403(c, "Forbidden: you don't have permission to delete objects in this bucket")
		return
	}

	// Get the object to verify it exists and belongs to this bucket
	object, err := ctrl.Repository.ObjectRepo.FindByID(objectID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Object not found: %v", err)
		utils.JSON404(c, "Object not found")
		return
	}

	// Verify object belongs to the specified bucket
	if object.BucketID != bucketID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] Object %s does not belong to bucket %s", objectID, bucketID)
		utils.JSON404(c, "Object not found in this bucket")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Deleting object '%s' from bucket '%s'", objectID, bucket.Name)

	// Delete object from database
	err = ctrl.Repository.ObjectRepo.Delete(objectID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to delete object from database: %v", err)
		utils.JSON500(c, "Failed to delete object")
		return
	}

	// TODO: Optionally delete file from storage via upload service
	// For now, we just delete the metadata. The actual file may be kept for deduplication purposes.

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Successfully deleted object: %s", objectID)
	utils.JSON200(c, gin.H{
		"message":   "Object deleted successfully",
		"object_id": objectID,
	})
}
