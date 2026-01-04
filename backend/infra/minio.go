package infra

import (
	"context"
	"fmt"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/tnqbao/gau-cloud-orchestrator/config"
)

// MinIO built-in policies
const (
	PolicyReadWrite    = "readwrite"    // Full access to all buckets
	PolicyReadOnly     = "readonly"     // Read-only access to all buckets
	PolicyWriteOnly    = "writeonly"    // Write-only access to all buckets
	PolicyConsoleAdmin = "consoleAdmin" // Console admin access
	PolicyDiagnostics  = "diagnostics"  // Diagnostics access
)

type MinioClient struct {
	Admin    *madmin.AdminClient
	Client   *minio.Client
	Endpoint string
}

func InitMinioClient(cfg *config.EnvConfig) *MinioClient {
	endpoint := cfg.Minio.Endpoint
	if endpoint == "" {
		panic("MinIO endpoint is not configured")
	}

	rootUser := cfg.Minio.RootUser
	if rootUser == "" {
		panic("MinIO root user is not configured")
	}

	rootPassword := cfg.Minio.RootPassword
	if rootPassword == "" {
		panic("MinIO root password is not configured")
	}

	madminClient, err := madmin.New(endpoint, rootUser, rootPassword, false)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize MinIO admin client: %v", err))
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(rootUser, rootPassword, ""),
		Secure: false,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize MinIO client: %v", err))
	}

	return &MinioClient{
		Admin:    madminClient,
		Client:   minioClient,
		Endpoint: endpoint,
	}
}

func (m *MinioClient) CreateIAMUser(ctx context.Context, accessKey, secretKey string) error {
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("accessKey and secretKey cannot be empty")
	}

	err := m.Admin.AddUser(ctx, accessKey, secretKey)
	if err != nil {
		return fmt.Errorf("failed to create MinIO IAM user: %w", err)
	}

	return nil
}

// AttachPolicyToUser attaches a policy to an IAM user
func (m *MinioClient) AttachPolicyToUser(ctx context.Context, accessKey, policyName string) error {
	if accessKey == "" || policyName == "" {
		return fmt.Errorf("accessKey and policyName cannot be empty")
	}

	err := m.Admin.SetPolicy(ctx, policyName, accessKey, false)
	if err != nil {
		return fmt.Errorf("failed to attach policy to MinIO user: %w", err)
	}

	return nil
}

// CreateIAMUserWithPolicy creates a user and attaches a policy in one operation
func (m *MinioClient) CreateIAMUserWithPolicy(ctx context.Context, accessKey, secretKey, policyName string) error {
	// Create user first
	if err := m.CreateIAMUser(ctx, accessKey, secretKey); err != nil {
		return err
	}

	// Attach policy to user
	if err := m.AttachPolicyToUser(ctx, accessKey, policyName); err != nil {
		// Rollback: delete user if policy attachment fails
		_ = m.DeleteIAMUser(ctx, accessKey)
		return err
	}

	return nil
}

// DeleteIAMUser deletes an IAM user from MinIO
func (m *MinioClient) DeleteIAMUser(ctx context.Context, accessKey string) error {
	if accessKey == "" {
		return fmt.Errorf("accessKey cannot be empty")
	}

	err := m.Admin.RemoveUser(ctx, accessKey)
	if err != nil {
		return fmt.Errorf("failed to delete MinIO IAM user: %w", err)
	}

	return nil
}

func (m *MinioClient) GetIAMUser(ctx context.Context, accessKey string) (*madmin.UserInfo, error) {
	if accessKey == "" {
		return nil, fmt.Errorf("accessKey cannot be empty")
	}

	userInfo, err := m.Admin.GetUserInfo(ctx, accessKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get MinIO IAM user info: %w", err)
	}

	return &userInfo, nil
}

func (m *MinioClient) UpdateIAMUserSecret(ctx context.Context, accessKey, newSecretKey string) error {
	if accessKey == "" || newSecretKey == "" {
		return fmt.Errorf("accessKey and newSecretKey cannot be empty")
	}

	err := m.Admin.AddUser(ctx, accessKey, newSecretKey)
	if err != nil {
		return fmt.Errorf("failed to update MinIO IAM user secret: %w", err)
	}

	return nil
}

func (m *MinioClient) DisableIAMUser(ctx context.Context, accessKey string) error {
	if accessKey == "" {
		return fmt.Errorf("accessKey cannot be empty")
	}

	err := m.Admin.SetUserStatus(ctx, accessKey, madmin.AccountDisabled)
	if err != nil {
		return fmt.Errorf("failed to disable MinIO IAM user: %w", err)
	}

	return nil
}

func (m *MinioClient) EnableIAMUser(ctx context.Context, accessKey string) error {
	if accessKey == "" {
		return fmt.Errorf("accessKey cannot be empty")
	}

	err := m.Admin.SetUserStatus(ctx, accessKey, madmin.AccountEnabled)
	if err != nil {
		return fmt.Errorf("failed to enable MinIO IAM user: %w", err)
	}

	return nil
}

// AddCannedPolicy creates a custom policy in MinIO
func (m *MinioClient) AddCannedPolicy(ctx context.Context, policyName string, policyJSON []byte) error {
	if policyName == "" {
		return fmt.Errorf("policyName cannot be empty")
	}
	if len(policyJSON) == 0 {
		return fmt.Errorf("policyJSON cannot be empty")
	}

	err := m.Admin.AddCannedPolicy(ctx, policyName, policyJSON)
	if err != nil {
		return fmt.Errorf("failed to add canned policy: %w", err)
	}

	return nil
}

// CreateIAMUserWithCustomPolicy creates a user, creates a custom policy, and attaches it
func (m *MinioClient) CreateIAMUserWithCustomPolicy(ctx context.Context, accessKey, secretKey, policyName string, policyJSON []byte) error {
	// Create custom policy first
	if err := m.AddCannedPolicy(ctx, policyName, policyJSON); err != nil {
		return fmt.Errorf("failed to create custom policy: %w", err)
	}

	// Create user
	if err := m.CreateIAMUser(ctx, accessKey, secretKey); err != nil {
		// Rollback: delete policy if user creation fails
		_ = m.DeletePolicy(ctx, policyName)
		return err
	}

	// Attach policy to user
	if err := m.AttachPolicyToUser(ctx, accessKey, policyName); err != nil {
		// Rollback: delete user and policy
		_ = m.DeleteIAMUser(ctx, accessKey)
		_ = m.DeletePolicy(ctx, policyName)
		return err
	}

	return nil
}

// DeletePolicy removes a policy from MinIO
func (m *MinioClient) DeletePolicy(ctx context.Context, policyName string) error {
	if policyName == "" {
		return fmt.Errorf("policyName cannot be empty")
	}

	err := m.Admin.RemoveCannedPolicy(ctx, policyName)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	return nil
}

// ListIAMUsers lists all IAM users in MinIO
func (m *MinioClient) ListIAMUsers(ctx context.Context) (map[string]madmin.UserInfo, error) {
	users, err := m.Admin.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list MinIO IAM users: %w", err)
	}

	return users, nil
}

func (m *MinioClient) SetIAMUserPolicy(ctx context.Context, accessKey, policyName string) error {
	if accessKey == "" || policyName == "" {
		return fmt.Errorf("accessKey and policyName cannot be empty")
	}

	err := m.Admin.SetPolicy(ctx, policyName, accessKey, false)
	if err != nil {
		return fmt.Errorf("failed to set MinIO IAM user policy: %w", err)
	}

	return nil
}

// CreateBucket creates a new bucket in MinIO
func (m *MinioClient) CreateBucket(ctx context.Context, bucketName, region string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	err := m.Client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region: region,
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// DeleteBucket deletes a bucket from MinIO
func (m *MinioClient) DeleteBucket(ctx context.Context, bucketName string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	err := m.Client.RemoveBucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

// RemoveAllObjectsFromBucket removes all objects from a bucket
func (m *MinioClient) RemoveAllObjectsFromBucket(ctx context.Context, bucketName string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	// List all objects in the bucket
	objectsCh := m.Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	// Create a channel for objects to delete
	objectsToDelete := make(chan minio.ObjectInfo)

	// Start goroutine to send objects to delete channel
	go func() {
		defer close(objectsToDelete)
		for object := range objectsCh {
			if object.Err != nil {
				continue
			}
			objectsToDelete <- object
		}
	}()

	// Remove objects
	errorCh := m.Client.RemoveObjects(ctx, bucketName, objectsToDelete, minio.RemoveObjectsOptions{})

	// Check for errors
	for err := range errorCh {
		if err.Err != nil {
			return fmt.Errorf("failed to remove object %s: %w", err.ObjectName, err.Err)
		}
	}

	return nil
}

// DeleteBucketWithObjects deletes a bucket and all its objects
func (m *MinioClient) DeleteBucketWithObjects(ctx context.Context, bucketName string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	// First, remove all objects from the bucket
	if err := m.RemoveAllObjectsFromBucket(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to remove objects from bucket: %w", err)
	}

	// Then delete the bucket
	if err := m.DeleteBucket(ctx, bucketName); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

// BucketExists checks if a bucket exists in MinIO
func (m *MinioClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	if bucketName == "" {
		return false, fmt.Errorf("bucketName cannot be empty")
	}

	exists, err := m.Client.BucketExists(ctx, bucketName)
	if err != nil {
		return false, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	return exists, nil
}

// SetBucketPolicy sets the access policy for a bucket (public or private)
func (m *MinioClient) SetBucketPolicy(ctx context.Context, bucketName, policy string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	var policyJSON string

	switch policy {
	case "public":
		// Public read policy - allows anonymous read access
		policyJSON = fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {"AWS": ["*"]},
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/*"]
				},
				{
					"Effect": "Allow",
					"Principal": {"AWS": ["*"]},
					"Action": ["s3:ListBucket"],
					"Resource": ["arn:aws:s3:::%s"]
				}
			]
		}`, bucketName, bucketName)
	case "private":
		// Private policy - no anonymous access (empty policy)
		policyJSON = ""
	default:
		return fmt.Errorf("invalid policy: %s. Must be 'public' or 'private'", policy)
	}

	err := m.Client.SetBucketPolicy(ctx, bucketName, policyJSON)
	if err != nil {
		return fmt.Errorf("failed to set bucket policy: %w", err)
	}

	return nil
}

// GetBucketPolicy retrieves the current access policy for a bucket
func (m *MinioClient) GetBucketPolicy(ctx context.Context, bucketName string) (string, error) {
	if bucketName == "" {
		return "", fmt.Errorf("bucketName cannot be empty")
	}

	policyJSON, err := m.Client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		// If no policy is set, it's private
		return "private", nil
	}

	if policyJSON == "" {
		return "private", nil
	}

	// Simple check - if policy allows public access, it's public
	// A more sophisticated check could parse the JSON
	return "public", nil
}

// EnsureBucket creates a bucket if it doesn't exist
func (m *MinioClient) EnsureBucket(ctx context.Context, bucketName string) error {
	exists, err := m.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}

	if !exists {
		if err := m.CreateBucket(ctx, bucketName, "us-east-1"); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}

// DeleteObjectsWithPrefix deletes all objects with a given prefix in a bucket
func (m *MinioClient) DeleteObjectsWithPrefix(ctx context.Context, bucketName, prefix string) error {
	objectCh := m.Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	objectsCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectsCh)
		for obj := range objectCh {
			if obj.Err != nil {
				continue
			}
			objectsCh <- obj
		}
	}()

	errorCh := m.Client.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{})

	for err := range errorCh {
		if err.Err != nil {
			return fmt.Errorf("failed to delete object %s: %w", err.ObjectName, err.Err)
		}
	}

	return nil
}
