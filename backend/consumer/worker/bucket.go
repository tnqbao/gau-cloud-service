package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"github.com/tnqbao/gau-cloud-orchestrator/infra"
	"github.com/tnqbao/gau-cloud-orchestrator/infra/produce"
	"github.com/tnqbao/gau-cloud-orchestrator/repository"
)

// MaskAccessKey masks access key for logging in consumer
func MaskAccessKey(accessKey string) string {
	if len(accessKey) <= 8 {
		return "***********"
	}
	return accessKey[:4] + "***********" + accessKey[len(accessKey)-4:]
}

type BucketConsumer struct {
	channel    *amqp.Channel
	infra      *infra.Infra
	repository *repository.Repository
}

func NewBucketConsumer(channel *amqp.Channel, infra *infra.Infra, repo *repository.Repository) *BucketConsumer {
	return &BucketConsumer{
		channel:    channel,
		infra:      infra,
		repository: repo,
	}
}

func (c *BucketConsumer) Start(ctx context.Context) error {
	if err := c.startUpdatePolicyConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start bucket update policy consumer: %w", err)
	}

	return nil
}

func (c *BucketConsumer) startUpdatePolicyConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.BucketUpdatePolicyQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register bucket update policy consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Started listening for update policy jobs on queue: %s", produce.BucketUpdatePolicyQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[Bucket Consumer - Update Policy] Channel closed")
					return
				}
				c.handleUpdatePolicy(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *BucketConsumer) handleUpdatePolicy(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] Received message: %s", string(msg.Body))

	var payload produce.UpdateBucketPolicyMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer - Update Policy] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer - Update Policy] Invalid User ID: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = c.executeUpdatePolicy(ctx, userID, payload.BucketName)
		if err == nil {
			c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] Successfully updated policies for user ID: %s, bucket: %s", userID.String(), payload.BucketName)
			_ = msg.Ack(false)
			return
		}

		c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer - Update Policy] Attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	// After max retries, reject and requeue
	c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer - Update Policy] Failed after %d attempts, requeueing message", maxRetries)
	_ = msg.Nack(false, true)
}

func (c *BucketConsumer) executeUpdatePolicy(ctx context.Context, userID uuid.UUID, bucketName string) error {
	// Step 1: Get all IAM users for this user_id
	iamUsers, err := c.repository.IAMUserRepo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("failed to get IAM users for user_id %s: %w", userID.String(), err)
	}

	if len(iamUsers) == 0 {
		c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] No IAM users found for user_id: %s", userID.String())
		return nil
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] Found %d IAM users for user_id: %s", len(iamUsers), userID.String())

	// Step 2: For each IAM user, get their policy and update it with the new bucket
	for _, iamUser := range iamUsers {
		if err := c.updateIAMUserPolicy(ctx, &iamUser, bucketName); err != nil {
			c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer - Update Policy] Failed to update policy for IAM user %s: %v", MaskAccessKey(iamUser.AccessKey), err)
			// Continue with other IAM users even if one fails
			continue
		}
		c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] Successfully updated policy for IAM user: %s", MaskAccessKey(iamUser.AccessKey))
	}

	return nil
}

func (c *BucketConsumer) updateIAMUserPolicy(ctx context.Context, iamUser *entity.IAMUser, bucketName string) error {
	// Step 1: Get current policy from database (type "s3")
	policy, err := c.repository.IAMPolicyRepo.GetByIAMIDAndType(iamUser.ID, "s3")
	if err != nil {
		return fmt.Errorf("failed to get s3 policy for IAM ID %s: %w", iamUser.ID.String(), err)
	}

	// Step 2: Parse the current policy JSON
	var policyDoc map[string]interface{}
	if err := json.Unmarshal(policy.Policy, &policyDoc); err != nil {
		return fmt.Errorf("failed to unmarshal policy JSON: %w", err)
	}

	// Step 3: Update the policy to include the new bucket
	statements, ok := policyDoc["Statement"].([]interface{})
	if !ok {
		return fmt.Errorf("policy Statement is not an array")
	}

	// Add bucket ARN to all statements with Resource field
	bucketARN := fmt.Sprintf("arn:aws:s3:::%s", bucketName)
	bucketObjectARN := fmt.Sprintf("arn:aws:s3:::%s/*", bucketName)

	for i, stmt := range statements {
		statement, ok := stmt.(map[string]interface{})
		if !ok {
			continue
		}

		resources, ok := statement["Resource"].([]interface{})
		if !ok {
			continue
		}

		// Check if bucket is already in the resources
		bucketExists := false
		for _, res := range resources {
			if resStr, ok := res.(string); ok {
				if resStr == bucketARN || resStr == bucketObjectARN {
					bucketExists = true
					break
				}
			}
		}

		if !bucketExists {
			// Add both bucket and bucket/* to resources
			resources = append(resources, bucketARN, bucketObjectARN)
			statement["Resource"] = resources
			statements[i] = statement
		}
	}

	policyDoc["Statement"] = statements

	// Step 4: Marshal updated policy
	updatedPolicyJSON, err := json.Marshal(policyDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal updated policy: %w", err)
	}

	// Step 5: Update policy on MinIO
	policyName := iamUser.AccessKey + "-s3-policy"

	// Delete old policy
	if err := c.infra.Minio.DeletePolicy(ctx, policyName); err != nil {
		c.infra.Logger.WarningWithContextf(ctx, "[Bucket Consumer - Update Policy] Failed to delete old policy (may not exist): %v", err)
	}

	// Create new policy with updated resources
	if err := c.infra.Minio.AddCannedPolicy(ctx, policyName, updatedPolicyJSON); err != nil {
		return fmt.Errorf("failed to create updated policy on MinIO: %w", err)
	}

	// Attach policy to user (reattach to ensure it's applied)
	if err := c.infra.Minio.AttachPolicyToUser(ctx, iamUser.AccessKey, policyName); err != nil {
		return fmt.Errorf("failed to attach updated policy to user: %w", err)
	}

	// Step 6: Update policy in database
	policy.Policy = updatedPolicyJSON
	if err := c.repository.IAMPolicyRepo.Update(policy); err != nil {
		return fmt.Errorf("failed to update policy in database: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer - Update Policy] Successfully updated policy for IAM user %s with bucket %s", MaskAccessKey(iamUser.AccessKey), bucketName)

	return nil
}
