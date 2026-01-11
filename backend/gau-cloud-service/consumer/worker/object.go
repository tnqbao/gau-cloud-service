package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tnqbao/gau-cloud-service/infra"
	"github.com/tnqbao/gau-cloud-service/infra/produce"
	"github.com/tnqbao/gau-cloud-service/repository"
)

// ObjectConsumer handles object deletion messages from the queue
type ObjectConsumer struct {
	channel    *amqp.Channel
	infra      *infra.Infra
	repository *repository.Repository
}

// NewObjectConsumer creates a new ObjectConsumer instance
func NewObjectConsumer(channel *amqp.Channel, infra *infra.Infra, repo *repository.Repository) *ObjectConsumer {
	return &ObjectConsumer{
		channel:    channel,
		infra:      infra,
		repository: repo,
	}
}

// Start begins consuming object deletion messages
func (c *ObjectConsumer) Start(ctx context.Context) error {
	if err := c.startDeleteObjectConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start object delete consumer: %w", err)
	}

	if err := c.startDeletePathConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start path delete consumer: %w", err)
	}

	return nil
}

func (c *ObjectConsumer) startDeleteObjectConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.ObjectDeleteQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register object delete consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer] Started listening for delete object jobs on queue: %s", produce.ObjectDeleteQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer - Delete Object] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[Object Consumer - Delete Object] Channel closed")
					return
				}
				c.handleDeleteObject(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *ObjectConsumer) handleDeleteObject(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer - Delete Object] Received message: %s", string(msg.Body))

	var payload produce.DeleteObjectMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Object Consumer - Delete Object] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	maxRetries := 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = c.infra.Minio.DeleteObject(ctx, payload.BucketName, payload.ObjectPath)
		if lastErr == nil {
			c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer - Delete Object] Successfully deleted object '%s' from bucket '%s'", payload.ObjectPath, payload.BucketName)
			_ = msg.Ack(false)
			return
		}

		c.infra.Logger.ErrorWithContextf(ctx, lastErr, "[Object Consumer - Delete Object] Attempt %d/%d failed: %v", attempt, maxRetries, lastErr)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	// After max retries, reject and requeue
	c.infra.Logger.ErrorWithContextf(ctx, lastErr, "[Object Consumer - Delete Object] Failed after %d attempts, requeueing message", maxRetries)
	_ = msg.Nack(false, true)
}

func (c *ObjectConsumer) startDeletePathConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.PathDeleteQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register path delete consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer] Started listening for delete path jobs on queue: %s", produce.PathDeleteQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer - Delete Path] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[Object Consumer - Delete Path] Channel closed")
					return
				}
				c.handleDeletePath(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *ObjectConsumer) handleDeletePath(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer - Delete Path] Received message: %s", string(msg.Body))

	var payload produce.DeletePathMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Object Consumer - Delete Path] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	maxRetries := 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Delete all objects with the given path prefix from MinIO
		lastErr = c.infra.Minio.DeleteObjectsWithPrefix(ctx, payload.BucketName, payload.Path+"/")
		if lastErr == nil {
			c.infra.Logger.InfoWithContextf(ctx, "[Object Consumer - Delete Path] Successfully deleted path '%s/' from bucket '%s'", payload.Path, payload.BucketName)
			_ = msg.Ack(false)
			return
		}

		c.infra.Logger.ErrorWithContextf(ctx, lastErr, "[Object Consumer - Delete Path] Attempt %d/%d failed: %v", attempt, maxRetries, lastErr)

		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}

	// After max retries, reject and requeue
	c.infra.Logger.ErrorWithContextf(ctx, lastErr, "[Object Consumer - Delete Path] Failed after %d attempts, requeueing message", maxRetries)
	_ = msg.Nack(false, true)
}
