package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"github.com/tnqbao/gau-cloud-orchestrator/infra/produce"
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

// handleSmallFileUpload handles files under the threshold using direct upload
func (ctrl *Controller) handleSmallFileUpload(c *gin.Context, fileHeader *multipart.FileHeader, bucket *entity.Bucket, bucketID uuid.UUID, customPath, contentType string) {
	ctx := c.Request.Context()

	// Open the file for reading
	file, err := fileHeader.Open()
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to open file")
		utils.JSON500(c, "Failed to open file")
		return
	}
	defer file.Close()

	// Forward to upload service
	uploadResponse, err := ctrl.Infra.UploadService.UploadFile(
		file,
		fileHeader.Filename,
		contentType,
		bucket.Name,
		customPath,
	)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to upload file to upload service: %v", err)
		utils.JSON500(c, "Failed to upload file: "+err.Error())
		return
	}

	// Extract URL from upload response (hash.ext format)
	urlPart := filepath.Base(uploadResponse.FilePath)

	// Create object entity with info from upload response
	object := &entity.Object{
		ID:           uuid.New(),
		BucketID:     bucketID,
		ContentType:  uploadResponse.ContentType,
		OriginName:   fileHeader.Filename,
		ParentPath:   customPath,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Size:         uploadResponse.Size,
		URL:          urlPart,
		FileHash:     uploadResponse.FileHash,
	}

	// Save object to database
	err = ctrl.Repository.ObjectRepo.Create(object)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to save object to database: %v", err)
		utils.JSON500(c, "Failed to save object metadata")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Successfully uploaded object: %s", object.ID)

	// Build CDN URL for the uploaded file
	cdnURL := ctrl.Infra.UploadService.GetCDNURL(bucket.Name, uploadResponse.FilePath)

	utils.JSON200(c, gin.H{
		"message":    "File uploaded successfully",
		"object":     object,
		"cdn_url":    cdnURL,
		"duplicated": uploadResponse.Duplicated,
	})
}

// handleLargeFileUpload handles files over the threshold using temp MinIO and chunking
func (ctrl *Controller) handleLargeFileUpload(c *gin.Context, fileHeader *multipart.FileHeader, bucket *entity.Bucket, bucketID, userID uuid.UUID, customPath, contentType string) {
	ctx := c.Request.Context()

	// Check if TempMinio is available
	if ctrl.Infra.TempMinio == nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, nil, "[Object] TempMinio not configured, cannot handle large file upload")
		utils.JSON500(c, "Large file upload is not configured")
		return
	}

	// Open the file for reading
	srcFile, err := fileHeader.Open()
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to open file")
		utils.JSON500(c, "Failed to open file")
		return
	}
	defer srcFile.Close()

	// Create a temporary file to store the content while hashing
	// Use TempMinio specific temp dir if configured, or default system temp
	// For orchestrator, we'll use system temp
	tempFile, err := os.CreateTemp("", "orchestrator-upload-*")
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to create temp file")
		utils.JSON500(c, "Failed to create temp file")
		return
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name()) // Clean up temp file
	}()

	// Create hasher
	hasher := sha256.New()

	// Stream from source to both temp file and hasher
	writer := io.MultiWriter(tempFile, hasher)

	// Copy data
	if _, err := io.Copy(writer, srcFile); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to stream file")
		utils.JSON500(c, "Failed to process file")
		return
	}

	// Calculate file hash
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// Get file extension
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".bin"
	}

	// Construct temp path with hash
	tempPath := fmt.Sprintf("%s%s", fileHash, ext)
	tempBucket := ctrl.Config.EnvConfig.LargeFile.TempBucket

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Large file detected (%d bytes), uploading to temp MinIO: %s/%s",
		fileHeader.Size, tempBucket, tempPath)

	// Ensure temp bucket exists
	if err := ctrl.Infra.TempMinio.EnsureBucket(ctx, tempBucket); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to ensure temp bucket exists")
		utils.JSON500(c, "Failed to prepare temp storage")
		return
	}

	// Reset temp file pointer to beginning for upload
	if _, err := tempFile.Seek(0, 0); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to seek temp file")
		utils.JSON500(c, "Failed to process file")
		return
	}

	// Upload to temp MinIO using stream
	metadata := map[string]string{
		"file-hash":     fileHash,
		"original-name": fileHeader.Filename,
		"content-type":  contentType,
		"user-id":       userID.String(),
		"bucket-id":     bucketID.String(),
	}

	if err := ctrl.Infra.TempMinio.PutObjectStream(ctx, tempBucket, tempPath, tempFile, fileHeader.Size, contentType, metadata); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to upload to temp MinIO")
		utils.JSON500(c, "Failed to upload file to temp storage")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] File uploaded to temp MinIO, publishing chunked upload message")

	// Construct target folder for chunks
	var targetFolder string
	if customPath != "" {
		targetFolder = fmt.Sprintf("%s/%s", customPath, fileHash)
	} else {
		targetFolder = fileHash
	}

	// Publish message to RabbitMQ for consumer processing
	msg := produce.ChunkedUploadMessage{
		UploadType:   getUploadType(ext),
		TempBucket:   tempBucket,
		TempPath:     tempPath,
		TargetBucket: bucket.Name,
		TargetFolder: targetFolder,
		OriginalName: fileHeader.Filename,
		FileHash:     fileHash,
		FileSize:     fileHeader.Size,
		ChunkSize:    0, // Use default chunk size
		Metadata: map[string]string{
			"user_id":      userID.String(),
			"bucket_id":    bucketID.String(),
			"content_type": contentType,
			"custom_path":  customPath,
		},
	}

	if err := ctrl.Infra.Produce.UploadService.PublishChunkedUpload(ctx, msg); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to publish chunked upload message")
		// Clean up temp file
		_ = ctrl.Infra.TempMinio.DeleteObject(ctx, tempBucket, tempPath)
		utils.JSON500(c, "Failed to queue file for processing")
		return
	}

	// Create object entity - URL will be the final file path (hash.ext)
	// Consumer will upload the file to: customPath/hash.ext or hash.ext
	urlPart := fmt.Sprintf("%s%s", fileHash, ext)
	object := &entity.Object{
		ID:           uuid.New(),
		BucketID:     bucketID,
		ContentType:  contentType,
		OriginName:   fileHeader.Filename,
		ParentPath:   customPath,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Size:         fileHeader.Size,
		URL:          urlPart, // File name: hash.ext
		FileHash:     fileHash,
	}

	// Save object to database
	err = ctrl.Repository.ObjectRepo.Create(object)
	if err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to save object to database: %v", err)
		utils.JSON500(c, "Failed to save object metadata")
		return
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Large file queued for chunking: %s (hash: %s)", object.ID, fileHash)

	utils.JSON202(c, gin.H{
		"message":       "Large file accepted for processing",
		"object":        object,
		"status":        "processing",
		"file_hash":     fileHash,
		"target_folder": targetFolder,
	})
}

// getUploadType returns the upload type based on file extension
func getUploadType(ext string) string {
	switch strings.ToLower(ext) {
	case ".zip":
		return "zip"
	case ".tar", ".tar.gz", ".tgz":
		return "archive"
	case ".mp4", ".avi", ".mov", ".mkv":
		return "video"
	case ".mp3", ".wav", ".flac":
		return "audio"
	default:
		return "file"
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
