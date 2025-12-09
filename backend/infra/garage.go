package infra

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	appConfig "github.com/tnqbao/gau-cloud-orchestrator/config"
)

type GarageClient struct {
	Client   *minio.Client
	Endpoint string
}

func InitGarageClient(cfg *appConfig.EnvConfig) *GarageClient {
	endpoint := cfg.Minio.Endpoint
	if endpoint == "" {
		panic("Garage endpoint is not configured")
	}

	accessKey := cfg.Minio.RootUser
	if accessKey == "" {
		panic("Garage access key is not configured")
	}

	secretKey := cfg.Minio.RootPassword
	if secretKey == "" {
		panic("Garage secret key is not configured")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // Set to true if using HTTPS
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize Garage client: %v", err))
	}

	return &GarageClient{
		Client:   client,
		Endpoint: endpoint,
	}
}

// CreateBucket creates a new bucket in Garage
func (g *GarageClient) CreateBucket(ctx context.Context, bucketName, region string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	if region == "" {
		region = "garage"
	}

	err := g.Client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region: region,
	})
	if err != nil {
		// Check if bucket already exists
		exists, errBucketExists := g.Client.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			return nil // Bucket already exists, not an error
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// DeleteBucket deletes a bucket from Garage
func (g *GarageClient) DeleteBucket(ctx context.Context, bucketName string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	err := g.Client.RemoveBucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

// RemoveAllObjectsFromBucket removes all objects from a bucket
func (g *GarageClient) RemoveAllObjectsFromBucket(ctx context.Context, bucketName string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	// List all objects
	objectsCh := g.Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	// Remove each object
	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("failed to list objects: %w", object.Err)
		}

		err := g.Client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("failed to remove object %s: %w", object.Key, err)
		}
	}

	return nil
}

// BucketExists checks if a bucket exists
func (g *GarageClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	if bucketName == "" {
		return false, fmt.Errorf("bucketName cannot be empty")
	}

	return g.Client.BucketExists(ctx, bucketName)
}

// ListBuckets lists all buckets
func (g *GarageClient) ListBuckets(ctx context.Context) ([]string, error) {
	buckets, err := g.Client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	names := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		names = append(names, bucket.Name)
	}

	return names, nil
}

// SetBucketPolicy sets a bucket policy
func (g *GarageClient) SetBucketPolicy(ctx context.Context, bucketName string, policy string) error {
	if bucketName == "" {
		return fmt.Errorf("bucketName cannot be empty")
	}

	err := g.Client.SetBucketPolicy(ctx, bucketName, policy)
	if err != nil {
		return fmt.Errorf("failed to set bucket policy: %w", err)
	}

	return nil
}

// GetBucketPolicy gets a bucket policy
func (g *GarageClient) GetBucketPolicy(ctx context.Context, bucketName string) (string, error) {
	if bucketName == "" {
		return "", fmt.Errorf("bucketName cannot be empty")
	}

	policy, err := g.Client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to get bucket policy: %w", err)
	}

	return policy, nil
}

// PutObject uploads an object to a bucket
func (g *GarageClient) PutObject(ctx context.Context, bucketName, objectKey string, body io.Reader) error {
	if bucketName == "" || objectKey == "" {
		return fmt.Errorf("bucketName and objectKey cannot be empty")
	}

	_, err := g.Client.PutObject(ctx, bucketName, objectKey, body, -1, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

// GetObject retrieves an object from a bucket
func (g *GarageClient) GetObject(ctx context.Context, bucketName, objectKey string) (io.ReadCloser, error) {
	if bucketName == "" || objectKey == "" {
		return nil, fmt.Errorf("bucketName and objectKey cannot be empty")
	}

	output, err := g.Client.GetObject(ctx, bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return output, nil
}

// DeleteObject deletes an object from a bucket
func (g *GarageClient) DeleteObject(ctx context.Context, bucketName, objectKey string) error {
	if bucketName == "" || objectKey == "" {
		return fmt.Errorf("bucketName and objectKey cannot be empty")
	}

	err := g.Client.RemoveObject(ctx, bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// ListObjects lists objects in a bucket
func (g *GarageClient) ListObjects(ctx context.Context, bucketName, prefix string) ([]string, error) {
	if bucketName == "" {
		return nil, fmt.Errorf("bucketName cannot be empty")
	}

	objectsCh := g.Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var objects []string
	for object := range objectsCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}

		objects = append(objects, object.Key)
	}

	return objects, nil
}
