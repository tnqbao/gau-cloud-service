package infra

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/tnqbao/gau-cloud-orchestrator/config"
)

// TempMinioClient is a separate MinIO client for temporary large file storage
type TempMinioClient struct {
	Client   *minio.Client
	Endpoint string
}

// NewTempMinioClient creates a new MinIO client for temp storage
func NewTempMinioClient(cfg *config.EnvConfig) (*TempMinioClient, error) {
	endpoint := cfg.TempMinio.Endpoint
	accessKey := cfg.TempMinio.AccessKey
	secretKey := cfg.TempMinio.SecretKey
	useSSL := cfg.TempMinio.UseSSL

	if endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("temp MinIO configuration is incomplete")
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize temp MinIO client: %w", err)
	}

	return &TempMinioClient{
		Client:   minioClient,
		Endpoint: endpoint,
	}, nil
}

// EnsureBucket creates a bucket if it doesn't exist
func (m *TempMinioClient) EnsureBucket(ctx context.Context, bucket string) error {
	exists, err := m.Client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = m.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return nil
}

// PutObject uploads an object to temp MinIO
func (m *TempMinioClient) PutObject(ctx context.Context, bucket, key string, data io.Reader, contentType string, metadata map[string]string) error {
	// Read all data to get size
	buf := new(bytes.Buffer)
	size, err := io.Copy(buf, data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: metadata,
	}

	_, err = m.Client.PutObject(ctx, bucket, key, bytes.NewReader(buf.Bytes()), size, opts)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}
	return nil
}

// PutObjectStream uploads an object to temp MinIO using a stream
func (m *TempMinioClient) PutObjectStream(ctx context.Context, bucket, key string, data io.Reader, size int64, contentType string, metadata map[string]string) error {
	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: metadata,
	}

	_, err := m.Client.PutObject(ctx, bucket, key, data, size, opts)
	if err != nil {
		return fmt.Errorf("failed to put object stream: %w", err)
	}
	return nil
}

// GetObjectStream streams an object from temp MinIO without loading into memory
func (m *TempMinioClient) GetObjectStream(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	obj, err := m.Client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get object: %w", err)
	}

	stat, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("failed to stat object: %w", err)
	}

	return obj, stat.Size, nil
}

// GetObject retrieves an object from temp MinIO
func (m *TempMinioClient) GetObject(ctx context.Context, bucket, key string) ([]byte, string, error) {
	obj, err := m.Client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat object: %w", err)
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, obj); err != nil {
		return nil, "", fmt.Errorf("failed to read object: %w", err)
	}

	return buf.Bytes(), stat.ContentType, nil
}

// DeleteObject deletes an object from temp MinIO
func (m *TempMinioClient) DeleteObject(ctx context.Context, bucket, key string) error {
	err := m.Client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// HeadObject checks if an object exists and gets its metadata
func (m *TempMinioClient) HeadObject(ctx context.Context, bucket, key string) (*minio.ObjectInfo, error) {
	stat, err := m.Client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}
	return &stat, nil
}
