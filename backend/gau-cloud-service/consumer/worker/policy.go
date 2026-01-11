package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tnqbao/gau-cloud-service/infra"
	"github.com/tnqbao/gau-cloud-service/infra/produce"
	"github.com/tnqbao/gau-cloud-service/repository"
)

type IAMConsumer struct {
	channel    *amqp.Channel
	infra      *infra.Infra
	repository *repository.Repository
}

func NewIAMConsumer(channel *amqp.Channel, infra *infra.Infra, repo *repository.Repository) *IAMConsumer {
	return &IAMConsumer{
		channel:    channel,
		infra:      infra,
		repository: repo,
	}
}

func (c *IAMConsumer) Start(ctx context.Context) error {
	if err := c.startUpdatePolicyConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start update policy consumer: %w", err)
	}

	return nil
}

func (c *IAMConsumer) startUpdatePolicyConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.IAMUpdatePolicyQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register update policy consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[IAM Consumer] Started listening for update policy jobs on queue: %s", produce.IAMUpdatePolicyQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[IAM Consumer - Update Policy] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[IAM Consumer - Update Policy] Channel closed")
					return
				}
				c.handleUpdatePolicy(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *IAMConsumer) handleUpdatePolicy(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[IAM Consumer - Update Policy] Received message: %s", string(msg.Body))

	var payload produce.UpdateIAMPolicyMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[IAM Consumer - Update Policy] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	iamID, err := uuid.Parse(payload.IAMID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[IAM Consumer - Update Policy] Invalid IAM ID: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = c.executeUpdatePolicy(ctx, iamID, payload.OldPolicyName, payload.NewPolicyName, payload.PolicyJSON)
		if err == nil {
			c.infra.Logger.InfoWithContextf(ctx, "[IAM Consumer - Update Policy] Successfully updated policy for IAM ID: %s", iamID.String())
			_ = msg.Ack(false)
			return
		}

		c.infra.Logger.ErrorWithContextf(ctx, err, "[IAM Consumer - Update Policy] Attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	// After max retries, reject and requeue
	c.infra.Logger.ErrorWithContextf(ctx, err, "[IAM Consumer - Update Policy] Failed after %d attempts, requeueing message", maxRetries)
	_ = msg.Nack(false, true)
}

func (c *IAMConsumer) executeUpdatePolicy(ctx context.Context, iamID uuid.UUID, oldPolicyName, newPolicyName string, policyJSON []byte) error {
	// Step 1: Delete old policy from MinIO
	if err := c.infra.Minio.DeletePolicy(ctx, oldPolicyName); err != nil {
		return fmt.Errorf("failed to delete old policy from MinIO: %w", err)
	}

	// Step 2: Create new policy with new name on MinIO
	if err := c.infra.Minio.AddCannedPolicy(ctx, newPolicyName, policyJSON); err != nil {
		// Rollback: Recreate old policy
		_ = c.infra.Minio.AddCannedPolicy(ctx, oldPolicyName, policyJSON)
		return fmt.Errorf("failed to create new policy on MinIO: %w", err)
	}

	// Step 3: Attach new policy to IAM user
	// Get IAM user to get access key
	iamUser, err := c.repository.IAMUserRepo.GetByID(iamID)
	if err != nil {
		// Rollback
		_ = c.infra.Minio.DeletePolicy(ctx, newPolicyName)
		_ = c.infra.Minio.AddCannedPolicy(ctx, oldPolicyName, policyJSON)
		return fmt.Errorf("failed to get IAM user: %w", err)
	}

	// Attach new policy to user
	if err := c.infra.Minio.AttachPolicyToUser(ctx, iamUser.AccessKey, newPolicyName); err != nil {
		// Rollback
		_ = c.infra.Minio.DeletePolicy(ctx, newPolicyName)
		_ = c.infra.Minio.AddCannedPolicy(ctx, oldPolicyName, policyJSON)
		_ = c.infra.Minio.AttachPolicyToUser(ctx, iamUser.AccessKey, oldPolicyName)
		return fmt.Errorf("failed to attach new policy to user: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[IAM Consumer - Update Policy] Successfully updated policy from %s to %s for user %s", oldPolicyName, newPolicyName, iamUser.AccessKey)

	return nil
}
