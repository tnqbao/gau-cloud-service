package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"github.com/tnqbao/gau-cloud-orchestrator/infra/produce"
	"github.com/tnqbao/gau-cloud-orchestrator/utils"
)

func BuildPolicyJSON(role string) []byte {
	switch role {
	case "admin":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:CreateBucket",
				"s3:DeleteBucket",
				"s3:ListAllMyBuckets",
				"s3:GetBucketLocation",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::*"]
		},
		{
			"Effect": "Allow",
			"Action": ["s3:*"],
			"Resource": ["arn:aws:s3:::*/*"]
		}
	]
}`)
	case "user":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": ["s3:CreateBucket"],
			"Resource": ["arn:aws:s3:::*"]
		},
		{
			"Effect": "Allow",
			"Action": [
				"s3:ListAllMyBuckets",
				"s3:ListBucket",
				"s3:GetBucketLocation",
				"s3:DeleteBucket"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		},
		{
			"Effect": "Allow",
			"Action": [
				"s3:GetObject",
				"s3:PutObject",
				"s3:DeleteObject"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket/*"]
		}
	]
}`)
	case "viewer":
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:ListAllMyBuckets",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::dummy-bucket"]
		},
		{
			"Effect": "Allow",
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::dummy-bucket/*"]
		}
	]
}`)
	default:
		return []byte(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"s3:CreateBucket",
				"s3:DeleteBucket",
				"s3:ListAllMyBuckets",
				"s3:GetBucketLocation",
				"s3:ListBucket"
			],
			"Resource": ["arn:aws:s3:::*"]
		},
		{
			"Effect": "Allow",
			"Action": ["s3:*"],
			"Resource": ["arn:aws:s3:::*/*"]
		}
	]
}`)
	}
}

// Object
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

	// Get file extension
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".bin"
	}

	tempBucket := ctrl.Config.EnvConfig.LargeFile.TempBucket

	// Ensure temp bucket exists before streaming
	if err := ctrl.Infra.TempMinio.EnsureBucket(ctx, tempBucket); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to ensure temp bucket '%s' exists: %v", tempBucket, err)
		utils.JSON500(c, fmt.Sprintf("Failed to prepare temp storage: %v", err))
		return
	}

	// Create hasher for SHA256
	hasher := sha256.New()

	// Use TeeReader to hash while streaming directly to MinIO (zero disk I/O)
	tee := io.TeeReader(srcFile, hasher)

	// Generate a temporary unique path for initial upload (will rename after hash is known)
	tempUploadID := uuid.New().String()
	tempPath := fmt.Sprintf("pending/%s%s", tempUploadID, ext)

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] Large file detected (%d bytes), streaming to temp MinIO: %s/%s",
		fileHeader.Size, tempBucket, tempPath)

	// Prepare metadata (hash will be added after upload)
	metadata := map[string]string{
		"original-name": fileHeader.Filename,
		"user-id":       userID.String(),
		"bucket-id":     bucketID.String(),
	}

	// Stream directly to temp MinIO while calculating hash
	if err := ctrl.Infra.TempMinio.PutObjectStream(ctx, tempBucket, tempPath, tee, fileHeader.Size, contentType, metadata); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to upload to temp MinIO")
		utils.JSON500(c, "Failed to upload file to temp storage")
		return
	}

	// Calculate file hash after streaming completes
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// Construct final temp path with hash
	finalTempPath := fmt.Sprintf("%s%s", fileHash, ext)

	// Move/copy object to final path with hash in name
	if err := ctrl.Infra.TempMinio.CopyObject(ctx, tempBucket, tempPath, tempBucket, finalTempPath); err != nil {
		ctrl.Infra.Logger.ErrorWithContextf(ctx, err, "[Object] Failed to rename temp object to final path")
		// Clean up the pending file
		_ = ctrl.Infra.TempMinio.DeleteObject(ctx, tempBucket, tempPath)
		utils.JSON500(c, "Failed to finalize temp upload")
		return
	}

	// Delete the original pending file
	if err := ctrl.Infra.TempMinio.DeleteObject(ctx, tempBucket, tempPath); err != nil {
		ctrl.Infra.Logger.WarningWithContextf(ctx, "[Object] Failed to delete pending temp file: %v", err)
		// Non-fatal, continue
	}

	ctrl.Infra.Logger.InfoWithContextf(ctx, "[Object] File streamed to temp MinIO with hash %s, publishing chunked upload message", fileHash)

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

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
