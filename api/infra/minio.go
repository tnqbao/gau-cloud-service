package infra

import (
	"context"
	"fmt"

	"github.com/minio/madmin-go/v3"
	"github.com/tnqbao/gau-cloud-orchestrator/config"
)

type MinioClient struct {
	Admin    *madmin.AdminClient
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

	return &MinioClient{
		Admin:    madminClient,
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
