package controller

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
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
