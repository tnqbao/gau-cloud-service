package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tnqbao/gau-cloud-service/entity"
	"github.com/tnqbao/gau-cloud-service/infra"
	"github.com/tnqbao/gau-cloud-service/infra/produce"
	"github.com/tnqbao/gau-cloud-service/repository"
)

type UploadConsumer struct {
	channel    *amqp.Channel
	infra      *infra.Infra
	repository *repository.Repository
}

func NewUploadConsumer(channel *amqp.Channel, infra *infra.Infra, repo *repository.Repository) *UploadConsumer {
	return &UploadConsumer{
		channel:    channel,
		infra:      infra,
		repository: repo,
	}
}

func (c *UploadConsumer) Start(ctx context.Context) error {
	if err := c.startComposeCompletedConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start upload consumer: %w", err)
	}
	return nil
}

// startComposeCompletedConsumer listens for compose_completed messages from upload-service
// After upload-service composes chunks and moves to final destination, it sends this message
// This consumer updates the database with the final object record
func (c *UploadConsumer) startComposeCompletedConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.ComposeCompletedQueue,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register compose_completed consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer] Started listening for compose_completed on queue: %s", produce.ComposeCompletedQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[Upload Consumer] Channel closed")
					return
				}
				c.handleComposeCompleted(ctx, msg)
			}
		}
	}()

	return nil
}

// handleComposeCompleted processes compose_completed messages from upload-service
// 1. Parse the message with file hash and final path
// 2. Create object record in database
// 3. Update upload session status to completed
func (c *UploadConsumer) handleComposeCompleted(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer] Received compose_completed message")

	var payload produce.ComposeCompletedMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer] Failed to unmarshal compose_completed message")
		_ = msg.Nack(false, false)
		return
	}

	uploadID, err := uuid.Parse(payload.UploadID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer] Invalid upload ID")
		_ = msg.Nack(false, false)
		return
	}

	bucketID, err := uuid.Parse(payload.BucketID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer] Invalid bucket ID")
		_ = msg.Nack(false, false)
		return
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer] Processing compose_completed for upload %s, success=%v", uploadID, payload.Success)

	// Check if compose was successful
	if !payload.Success {
		c.infra.Logger.ErrorWithContextf(ctx, nil, "[Upload Consumer] Compose failed: %s", payload.Error)
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Ack(false)
		return
	}

	// Update session with file hash
	if err := c.repository.UploadSessionRepo.UpdateFileHash(uploadID, payload.FileHash); err != nil {
		c.infra.Logger.WarningWithContextf(ctx, "[Upload Consumer] Failed to update file hash: %v", err)
	}

	// Get file extension from original filename
	ext := filepath.Ext(payload.FileName)
	if ext == "" {
		ext = ".bin"
	}

	// Construct URL part (hash + extension)
	urlPart := fmt.Sprintf("%s%s", payload.FileHash, ext)

	// Create object record in database
	object := &entity.Object{
		ID:           uuid.New(),
		BucketID:     bucketID,
		ContentType:  payload.ContentType,
		OriginName:   payload.FileName,
		ParentPath:   payload.CustomPath,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Size:         payload.FileSize,
		URL:          urlPart,
		FileHash:     payload.FileHash,
	}

	if err := c.repository.ObjectRepo.Create(object); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer] Failed to save object to database")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true)
		return
	}

	// Mark upload as completed
	c.updateSessionStatus(uploadID, entity.UploadStatusCompleted)

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer] Successfully completed upload %s, object %s created (hash: %s)",
		uploadID, object.ID, payload.FileHash)

	// Acknowledge the message
	_ = msg.Ack(false)
}

func (c *UploadConsumer) updateSessionStatus(uploadID uuid.UUID, status entity.UploadStatus) {
	if err := c.repository.UploadSessionRepo.UpdateStatus(uploadID, status); err != nil {
		c.infra.Logger.WarningWithContextf(context.Background(), "[Upload Consumer] Failed to update session status: %v", err)
	}
}
