package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tnqbao/gau-cloud-orchestrator/infra"
	"github.com/tnqbao/gau-cloud-orchestrator/infra/produce"
	"github.com/tnqbao/gau-cloud-orchestrator/repository"
)

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
	if err := c.startDeleteBucketConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start bucket delete consumer: %w", err)
	}

	return nil
}

func (c *BucketConsumer) startDeleteBucketConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.BucketDeleteQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register bucket delete consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Started listening for delete bucket jobs on queue: %s", produce.BucketDeleteQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[Bucket Consumer] Channel closed")
					return
				}
				c.handleDeleteBucket(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *BucketConsumer) handleDeleteBucket(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Received delete message: %s", string(msg.Body))

	var payload produce.DeleteBucketMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	_, err := uuid.Parse(payload.UserID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer] Invalid User ID: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = c.executeDeleteBucket(ctx, payload.BucketName)
		if err == nil {
			c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Successfully deleted bucket: %s", payload.BucketName)
			_ = msg.Ack(false)
			return
		}

		c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer] Attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	// After max retries, reject without requeue (dead letter)
	c.infra.Logger.ErrorWithContextf(ctx, err, "[Bucket Consumer] Failed after %d attempts, rejecting message", maxRetries)
	_ = msg.Nack(false, false)
}

func (c *BucketConsumer) executeDeleteBucket(ctx context.Context, bucketName string) error {
	// Step 1: Remove all objects from bucket
	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Removing all objects from bucket: %s", bucketName)

	err := c.infra.Garage.RemoveAllObjectsFromBucket(ctx, bucketName)
	if err != nil {
		// Bucket might not exist or already empty, log warning and continue
		c.infra.Logger.WarningWithContextf(ctx, "[Bucket Consumer] Failed to remove objects (may not exist): %v", err)
	}

	// Step 2: Delete the bucket from Garage
	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Deleting bucket from Garage: %s", bucketName)

	err = c.infra.Garage.DeleteBucket(ctx, bucketName)
	if err != nil {
		c.infra.Logger.WarningWithContextf(ctx, "[Bucket Consumer] Failed to delete bucket (may not exist): %v", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Bucket Consumer] Bucket cleanup completed: %s", bucketName)
	return nil
}
