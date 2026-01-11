package controller

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-service/entity"
	"github.com/tnqbao/gau-cloud-service/http/controller/dto"
	"github.com/tnqbao/gau-cloud-service/infra/produce"
	"github.com/tnqbao/gau-cloud-service/utils"
)

const (
	// DefaultChunkSize is the default size for each chunk (5MB)
	DefaultChunkSize int64 = 10 * 1024 * 1024
	// MaxChunkSize is the maximum allowed chunk size (10MB - safe for Cloudflare)
	MaxChunkSize int64 = 15 * 1024 * 1024
	// UploadSessionExpiry is the default expiry time for upload sessions (24 hours)
	UploadSessionExpiry = 24 * time.Hour
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
		// Large file: reject and instruct to use chunked upload API
		// This prevents Cloudflare 413 errors by ensuring large files use chunked upload
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] File size %d exceeds threshold %d, rejecting",
			fileHeader.Size, largeFileThreshold)
		utils.JSON413(c, gin.H{
			"error":     "FILE_TOO_LARGE",
			"message":   "File size exceeds the maximum allowed for direct upload",
			"hint":      "Use chunked upload API for files larger than " + formatBytes(largeFileThreshold),
			"file_size": fileHeader.Size,
			"threshold": largeFileThreshold,
			"endpoints": gin.H{
				"init":     "POST /api/v1/cloud/buckets/:id/chunked/init",
				"chunk":    "POST /api/v1/cloud/buckets/:id/chunked/chunk",
				"complete": "POST /api/v1/cloud/buckets/:id/chunked/complete",
			},
		})
		return
	}

	// Small file flow: use existing direct upload
	ctrl.handleSmallFileUpload(c, fileHeader, bucket, bucketID, customPath, contentType)
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

	// Store object URL for message before deletion
	objectURL := object.URL

	// Delete object from database
	err = ctrl.Repository.ObjectRepo.Delete(objectID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to delete object from database: %v", err)
		utils.JSON500(c, "Failed to delete object")
		return
	}

	// Publish message to consumer to delete object from MinIO storage
	deleteMsg := produce.DeleteObjectMessage{
		BucketName: bucket.Name,
		ObjectPath: objectURL, // URL field contains the hash.ext format path in MinIO
		UserID:     userIDStr,
	}
	if err := ctrl.Infra.Produce.UploadService.PublishDeleteObject(ctx, deleteMsg); err != nil {
		// Log the error but don't fail the request - DB record is already deleted
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to publish delete message for object %s: %v", objectID, err)
	} else {
		ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Published delete message for object %s in bucket %s", objectID, bucket.Name)
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Successfully deleted object: %s", objectID)
	utils.JSON200(c, gin.H{
		"message":   "Object deleted successfully",
		"object_id": objectID,
	})
}

// DownloadObject streams an object directly to the client without buffering in memory
// GET /buckets/:id/objects/:object_id/download
func (ctrl *Controller) DownloadObject(c *gin.Context) {
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
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] User %s attempted to download object from bucket %s owned by %s", userID, bucketID, bucket.OwnerID)
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
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

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Streaming download for object '%s' from bucket '%s'", objectID, bucket.Name)

	// Get object stream from MinIO
	minioObject, objectInfo, err := ctrl.Infra.Minio.GetObject(ctx, bucket.Name, object.URL)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to get object from MinIO: %v", err)
		utils.JSON500(c, "Failed to retrieve object")
		return
	}
	defer minioObject.Close()

	// Set response headers for download
	contentType := object.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", object.OriginName))
	c.Header("Content-Length", fmt.Sprintf("%d", objectInfo.Size))

	// Stream directly to client without buffering in RAM
	c.Status(200)
	_, err = io.Copy(c.Writer, minioObject)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to stream object to client: %v", err)
		// Can't send error response since we've already started writing
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Successfully streamed object '%s' (%d bytes)", objectID, objectInfo.Size)
}

// DeleteObjectsByPath deletes all objects in a path and sends message to consumer
// DELETE /buckets/:id/objects/path/*path
func (ctrl *Controller) DeleteObjectsByPath(c *gin.Context) {
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

	// Get path from wildcard parameter and normalize it
	deletePath := c.Param("path")
	// Remove leading slash from wildcard path
	deletePath = strings.TrimPrefix(deletePath, "/")
	// Remove trailing slash
	deletePath = strings.TrimSuffix(deletePath, "/")
	// Clean the path
	deletePath = strings.TrimSpace(deletePath)

	// Check if bucket exists and user has permission
	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Bucket not found: %v", err)
		utils.JSON404(c, "Bucket not found")
		return
	}

	// Check if the user owns this bucket
	if bucket.OwnerID != userID {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] User %s attempted to delete path in bucket %s owned by %s", userID, bucketID, bucket.OwnerID)
		utils.JSON403(c, "Forbidden: you don't have permission to delete objects in this bucket")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Deleting all objects at path '%s' in bucket '%s'", deletePath, bucket.Name)

	// Delete all objects with this path prefix from database and get hashes for cleanup
	deletedObjects, err := ctrl.Repository.ObjectRepo.DeleteByBucketIDAndPathPrefix(bucketID, deletePath)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to delete objects from database: %v", err)
		utils.JSON500(c, "Failed to delete objects")
		return
	}

	// Publish message to consumer to delete path from MinIO storage
	deleteMsg := produce.DeletePathMessage{
		BucketName: bucket.Name,
		Path:       deletePath,
		UserID:     userIDStr,
	}
	if err := ctrl.Infra.Produce.UploadService.PublishDeletePath(ctx, deleteMsg); err != nil {
		// Log the error but don't fail the request - DB records are already deleted
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to publish delete path message: %v", err)
	} else {
		ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Published delete path message for path '%s' in bucket '%s'", deletePath, bucket.Name)
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Successfully deleted %d objects at path '%s'", len(deletedObjects), deletePath)
	utils.JSON200(c, gin.H{
		"message":       "Objects deleted successfully",
		"path":          deletePath,
		"deleted_count": len(deletedObjects),
	})
}

// InitChunkedUpload initializes a chunked upload session for large files
func (ctrl *Controller) InitChunkedUpload(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Object] user_id not found in context")
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid user_id format")
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	bucketIDStr := c.Param("id")
	if bucketIDStr == "" {
		utils.JSON400(c, "bucket_id is required")
		return
	}

	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		utils.JSON404(c, "Bucket not found")
		return
	}

	if bucket.OwnerID != userID {
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	var req dto.InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Invalid request body")
		utils.JSON400(c, "Invalid request body: "+err.Error())
		return
	}

	largeFileThreshold := ctrl.Config.EnvConfig.LargeFile.Threshold
	if largeFileThreshold == 0 {
		largeFileThreshold = 52428800
	}

	if req.FileSize <= largeFileThreshold {
		utils.JSON400(c, fmt.Sprintf("File size is below threshold (%d bytes). Use regular upload endpoint.", largeFileThreshold))
		return
	}

	// Server decides chunk size (production-grade architecture)
	// 1. Start with default chunk size (5MB)
	chunkSize := DefaultChunkSize

	// 2. If client suggests a preferred chunk size, consider it
	if req.PreferredChunkSize > 0 {
		// Validate client's preference is within acceptable range
		if req.PreferredChunkSize < DefaultChunkSize {
			// Too small, use default
			ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Client preferred chunk size %d is below minimum, using default %d",
				req.PreferredChunkSize, DefaultChunkSize)
		} else if req.PreferredChunkSize > MaxChunkSize {
			// Too large, cap at maximum
			chunkSize = MaxChunkSize
			ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Client preferred chunk size %d exceeds maximum, capping at %d",
				req.PreferredChunkSize, MaxChunkSize)
		} else {
			// Within acceptable range, use client's preference
			chunkSize = req.PreferredChunkSize
			ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Using client preferred chunk size: %d", chunkSize)
		}
	}

	// 3. Calculate total chunks based on SERVER-DECIDED chunk size
	totalChunks := int((req.FileSize + chunkSize - 1) / chunkSize)

	uploadID := uuid.New()
	tempBucket := ctrl.Config.EnvConfig.LargeFile.TempBucket
	tempPrefix := fmt.Sprintf("pending/%s/", uploadID.String())

	customPath := strings.TrimSpace(req.Path)
	if customPath != "" {
		customPath = strings.Trim(customPath, "/\\")
		customPath = strings.ReplaceAll(customPath, "\\", "/")
		for strings.Contains(customPath, "//") {
			customPath = strings.ReplaceAll(customPath, "//", "/")
		}
		if strings.Contains(customPath, "..") {
			utils.JSON400(c, "Invalid path: path cannot contain '..'")
			return
		}
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := ctrl.Infra.Minio.EnsureBucket(ctx, tempBucket); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to ensure pending bucket")
		utils.JSON500(c, "Failed to prepare upload storage")
		return
	}

	session := &entity.UploadSession{
		ID:             uploadID,
		BucketID:       bucketID,
		UserID:         userID,
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		ContentType:    contentType,
		CustomPath:     customPath,
		ChunkSize:      chunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: 0,
		Status:         entity.UploadStatusInit,
		TempBucket:     tempBucket,
		TempPrefix:     tempPrefix,
		ExpiresAt:      time.Now().Add(UploadSessionExpiry),
	}

	if err := ctrl.Repository.UploadSessionRepo.Create(session); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to create upload session")
		utils.JSON500(c, "Failed to initialize upload session")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Initialized upload session %s for file '%s' (%d bytes, %d chunks of %d bytes each)",
		uploadID, req.FileName, req.FileSize, totalChunks, chunkSize)

	// Server returns the CONTRACT that client MUST follow
	utils.JSON200(c, gin.H{
		"upload_id":    uploadID.String(),
		"chunk_size":   chunkSize,   // Client MUST use this chunk size
		"total_chunks": totalChunks, // Expected number of chunks
		"temp_prefix":  tempPrefix,
		"expires_at":   session.ExpiresAt.Format(time.RFC3339),
	})
}

// UploadChunk handles uploading a single chunk
// POST /bucket/:id/uploads/chunk
func (ctrl *Controller) UploadChunk(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	bucketIDStr := c.Param("id")
	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		utils.JSON404(c, "Bucket not found")
		return
	}

	if bucket.OwnerID != userID {
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	uploadIDStr := c.Query("upload_id")
	if uploadIDStr == "" {
		uploadIDStr = c.GetHeader("X-Upload-ID")
	}
	if uploadIDStr == "" {
		utils.JSON400(c, "upload_id is required")
		return
	}

	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid upload_id format")
		return
	}

	chunkIndexStr := c.Query("chunk_index")
	if chunkIndexStr == "" {
		chunkIndexStr = c.GetHeader("X-Chunk-Index")
	}
	var chunkIndex int
	if _, err := fmt.Sscanf(chunkIndexStr, "%d", &chunkIndex); err != nil {
		utils.JSON400(c, "Invalid chunk_index")
		return
	}

	session, err := ctrl.Repository.UploadSessionRepo.FindByIDAndBucketID(uploadID, bucketID)
	if err != nil {
		utils.JSON404(c, "Upload session not found")
		return
	}

	if session.Status != entity.UploadStatusInit && session.Status != entity.UploadStatusUploading {
		utils.JSON400(c, fmt.Sprintf("Upload session is not active, current status: %s", session.Status))
		return
	}

	if time.Now().After(session.ExpiresAt) {
		utils.JSON400(c, "Upload session has expired")
		return
	}

	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		utils.JSON400(c, fmt.Sprintf("Invalid chunk_index: must be between 0 and %d", session.TotalChunks-1))
		return
	}

	// Check if UploadService is configured
	if ctrl.Infra.UploadService == nil {
		utils.JSON500(c, "Upload service is not configured")
		return
	}

	var chunkReader io.Reader
	var chunkSize int64

	file, header, err := c.Request.FormFile("chunk")
	if err == nil {
		chunkReader = file
		chunkSize = header.Size
		defer file.Close()
	} else {
		chunkReader = c.Request.Body
		chunkSize = c.Request.ContentLength
		if chunkSize <= 0 {
			utils.JSON400(c, "Content-Length header is required for raw body upload")
			return
		}
	}

	if chunkSize > MaxChunkSize {
		utils.JSON400(c, fmt.Sprintf("Chunk size %d exceeds maximum allowed %d", chunkSize, MaxChunkSize))
		return
	}

	// Use zero-padded chunk index to ensure correct sorting (e.g., chunk_00000, chunk_00001, ...)
	chunkFileName := fmt.Sprintf("chunk_%05d.part", chunkIndex)

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Uploading chunk %d/%d for session %s via upload-service (size: %d)",
		chunkIndex+1, session.TotalChunks, uploadID, chunkSize)

	// Upload chunk to upload-service with:
	// - bucket: "pending"
	// - path: "{upload_id}"
	// - is_hash: false (to preserve original chunk filename)
	uploadResp, err := ctrl.Infra.UploadService.UploadChunkToService(
		chunkReader,
		chunkFileName,
		"application/octet-stream",
		"pending",
		uploadID.String(),
	)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to upload chunk %d via upload-service", chunkIndex)
		utils.JSON500(c, "Failed to upload chunk")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Upload-service response: bucket=%s, path=%s, hash=%s",
		uploadResp.Bucket, uploadResp.FilePath, uploadResp.FileHash)

	if err := ctrl.Repository.UploadSessionRepo.IncrementUploadedChunks(uploadID); err != nil {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] Failed to update upload progress: %v", err)
	}

	updatedSession, _ := ctrl.Repository.UploadSessionRepo.GetUploadProgress(uploadID)
	uploadedChunks := 0
	if updatedSession != nil {
		uploadedChunks = updatedSession.UploadedChunks
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Chunk %d uploaded successfully via upload-service (%d/%d)",
		chunkIndex, uploadedChunks, session.TotalChunks)

	utils.JSON200(c, gin.H{
		"chunk_index":     chunkIndex,
		"uploaded_chunks": uploadedChunks,
		"total_chunks":    session.TotalChunks,
		"status":          string(entity.UploadStatusUploading),
		"file_path":       uploadResp.FilePath,
	})
}

// CompleteChunkedUpload completes a chunked upload by composing all chunks
// POST /bucket/:id/uploads/complete
func (ctrl *Controller) CompleteChunkedUpload(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	bucketIDStr := c.Param("id")
	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		utils.JSON404(c, "Bucket not found")
		return
	}

	if bucket.OwnerID != userID {
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	var req dto.CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.JSON400(c, "Invalid request body: "+err.Error())
		return
	}

	uploadID, err := uuid.Parse(req.UploadID)
	if err != nil {
		utils.JSON400(c, "Invalid upload_id format")
		return
	}

	session, err := ctrl.Repository.UploadSessionRepo.FindByIDAndBucketID(uploadID, bucketID)
	if err != nil {
		utils.JSON404(c, "Upload session not found")
		return
	}

	if session.Status != entity.UploadStatusInit && session.Status != entity.UploadStatusUploading {
		utils.JSON400(c, fmt.Sprintf("Upload session is not active, current status: %s", session.Status))
		return
	}

	if time.Now().After(session.ExpiresAt) {
		utils.JSON400(c, "Upload session has expired")
		return
	}

	// Verify all chunks have been uploaded by checking session uploaded count
	if session.UploadedChunks != session.TotalChunks {
		utils.JSON400(c, fmt.Sprintf("Missing chunks: expected %d, uploaded %d", session.TotalChunks, session.UploadedChunks))
		return
	}

	// Update session status to processing IMMEDIATELY (fast operation)
	if err := ctrl.Repository.UploadSessionRepo.UpdateStatus(uploadID, entity.UploadStatusProcessing); err != nil {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] Failed to update session status: %v", err)
	}

	// Publish message for async processing by upload-service
	// Upload-service will compose chunks and move to final destination
	msg := produce.ChunkCompleteMessage{
		UploadID:     uploadID.String(),
		BucketID:     bucketID.String(),
		BucketName:   bucket.Name,
		UserID:       userID.String(),
		TempBucket:   "pending",                             // Chunks are in "pending" bucket
		TempPrefix:   fmt.Sprintf("%s/", uploadID.String()), // Path is upload_id/
		FileName:     session.FileName,
		FileSize:     session.FileSize,
		ContentType:  session.ContentType,
		CustomPath:   session.CustomPath,
		TotalChunks:  session.TotalChunks,
		TargetBucket: bucket.Name,        // Final destination bucket
		TargetPath:   session.CustomPath, // Final destination path
		Metadata: map[string]string{
			"content_type": session.ContentType,
			"custom_path":  session.CustomPath,
		},
	}

	if err := ctrl.Infra.Produce.UploadService.PublishChunkComplete(ctx, msg); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to publish chunk_complete message to queue")
		// Revert status
		_ = ctrl.Repository.UploadSessionRepo.UpdateStatus(uploadID, entity.UploadStatusUploading)
		utils.JSON500(c, "Failed to queue file for processing")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Upload session %s queued for processing (%d chunks)", uploadID, session.TotalChunks)

	// Return immediately - client should poll for status
	utils.JSON202(c, gin.H{
		"message":      "Upload accepted for processing",
		"upload_id":    uploadID.String(),
		"status":       "processing",
		"total_chunks": session.TotalChunks,
		"file_name":    session.FileName,
		"file_size":    session.FileSize,
		"status_url":   fmt.Sprintf("/api/v1/cloud/buckets/%s/chunked/%s/status", bucketID, uploadID),
	})
}

// GetUploadProgress returns the current progress of an upload session
// GET /bucket/:id/uploads/:upload_id/progress
func (ctrl *Controller) GetUploadProgress(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	bucketIDStr := c.Param("id")
	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		utils.JSON404(c, "Bucket not found")
		return
	}

	if bucket.OwnerID != userID {
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	uploadIDStr := c.Param("upload_id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid upload_id format")
		return
	}

	session, err := ctrl.Repository.UploadSessionRepo.FindByIDAndBucketID(uploadID, bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Upload session not found: %s", uploadID)
		utils.JSON404(c, "Upload session not found")
		return
	}

	progress := float64(0)
	if session.TotalChunks > 0 {
		progress = float64(session.UploadedChunks) / float64(session.TotalChunks) * 100
	}

	utils.JSON200(c, gin.H{
		"upload_id":       session.ID.String(),
		"uploaded_chunks": session.UploadedChunks,
		"total_chunks":    session.TotalChunks,
		"status":          string(session.Status),
		"progress":        progress,
	})
}

// AbortChunkedUpload aborts an upload session and cleans up chunks
// DELETE /bucket/:id/uploads/:upload_id
func (ctrl *Controller) AbortChunkedUpload(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	bucketIDStr := c.Param("id")
	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		utils.JSON404(c, "Bucket not found")
		return
	}

	if bucket.OwnerID != userID {
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	uploadIDStr := c.Param("upload_id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid upload_id format")
		return
	}

	session, err := ctrl.Repository.UploadSessionRepo.FindByIDAndBucketID(uploadID, bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Upload session not found: %s", uploadID)
		utils.JSON404(c, "Upload session not found")
		return
	}

	go func() {
		// Cleanup chunks from pending bucket
		_ = ctrl.Infra.Minio.DeleteObjectsWithPrefix(ctx, session.TempBucket, session.TempPrefix)
	}()

	if err := ctrl.Repository.UploadSessionRepo.Delete(uploadID); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to delete upload session")
		utils.JSON500(c, "Failed to abort upload")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Upload session aborted: %s", uploadID)

	utils.JSON200(c, gin.H{
		"message":   "Upload aborted successfully",
		"upload_id": uploadID.String(),
	})
}

// GetChunkedUploadStatus returns detailed status of a chunked upload session
// GET /bucket/:id/chunked/:upload_id/status
func (ctrl *Controller) GetChunkedUploadStatus(c *gin.Context) {
	ctx := c.Request.Context()
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		utils.JSON401(c, "Unauthorized: user_id not found")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid user_id format")
		return
	}

	bucketIDStr := c.Param("id")
	bucketID, err := uuid.Parse(bucketIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid bucket_id format")
		return
	}

	bucket, err := ctrl.Repository.BucketRepo.FindByID(bucketID)
	if err != nil {
		utils.JSON404(c, "Bucket not found")
		return
	}

	if bucket.OwnerID != userID {
		utils.JSON403(c, "Forbidden: you don't have permission to access this bucket")
		return
	}

	uploadIDStr := c.Param("upload_id")
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		utils.JSON400(c, "Invalid upload_id format")
		return
	}

	session, err := ctrl.Repository.UploadSessionRepo.FindByIDAndBucketID(uploadID, bucketID)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Upload session not found: %s", uploadID)
		utils.JSON404(c, "Upload session not found")
		return
	}

	// Calculate upload progress
	uploadProgress := float64(0)
	if session.TotalChunks > 0 {
		uploadProgress = float64(session.UploadedChunks) / float64(session.TotalChunks) * 100
	}

	// Build response based on status
	response := gin.H{
		"upload_id":       session.ID.String(),
		"file_name":       session.FileName,
		"file_size":       session.FileSize,
		"content_type":    session.ContentType,
		"status":          string(session.Status),
		"uploaded_chunks": session.UploadedChunks,
		"total_chunks":    session.TotalChunks,
		"upload_progress": uploadProgress,
		"created_at":      session.CreatedAt,
		"updated_at":      session.UpdatedAt,
		"expires_at":      session.ExpiresAt,
	}

	// Add status-specific information
	switch session.Status {
	case entity.UploadStatusInit:
		response["message"] = "Upload session initialized, waiting for chunks"
		response["is_complete"] = false

	case entity.UploadStatusUploading:
		response["message"] = fmt.Sprintf("Uploading chunks: %d/%d", session.UploadedChunks, session.TotalChunks)
		response["is_complete"] = false

	case entity.UploadStatusProcessing:
		response["message"] = "All chunks uploaded, processing file (composing, hashing, copying)..."
		response["is_complete"] = false
		response["processing_steps"] = []string{
			"1. Composing chunks into single file",
			"2. Calculating SHA256 hash",
			"3. Moving to final storage",
			"4. Creating object record",
		}

	case entity.UploadStatusCompleted:
		response["message"] = "Upload completed successfully"
		response["is_complete"] = true
		response["file_hash"] = session.FileHash
		// Try to get the object info
		if session.FileHash != "" {
			objects, err := ctrl.Repository.ObjectRepo.FindByBucketIDAndHash(bucketID, session.FileHash)
			if err == nil && len(objects) > 0 {
				response["object"] = gin.H{
					"id":          objects[0].ID.String(),
					"url":         objects[0].URL,
					"file_hash":   objects[0].FileHash,
					"size":        objects[0].Size,
					"origin_name": objects[0].OriginName,
				}
			}
		}

	case entity.UploadStatusFailed:
		response["message"] = "Upload failed during processing"
		response["is_complete"] = true
		response["error"] = "An error occurred while processing the upload. Please try again."

	case entity.UploadStatusExpired:
		response["message"] = "Upload session has expired"
		response["is_complete"] = true
		response["error"] = "The upload session has expired. Please start a new upload."
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) && session.Status != entity.UploadStatusCompleted {
		response["status"] = string(entity.UploadStatusExpired)
		response["message"] = "Upload session has expired"
		response["is_complete"] = true
		response["error"] = "The upload session has expired. Please start a new upload."
	}

	utils.JSON200(c, response)
}
