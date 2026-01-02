package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"github.com/tnqbao/gau-cloud-orchestrator/infra"
	"github.com/tnqbao/gau-cloud-orchestrator/infra/produce"
	"github.com/tnqbao/gau-cloud-orchestrator/repository"
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
	if err := c.startComposeChunksConsumer(ctx); err != nil {
		return fmt.Errorf("failed to start compose chunks consumer: %w", err)
	}
	return nil
}

func (c *UploadConsumer) startComposeChunksConsumer(ctx context.Context) error {
	msgs, err := c.channel.Consume(
		produce.ComposeChunksQueue,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register compose chunks consumer: %w", err)
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer] Started listening for compose chunks jobs on queue: %s", produce.ComposeChunksQueue)

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer - Compose Chunks] Shutting down...")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.infra.Logger.WarningWithContextf(ctx, "[Upload Consumer - Compose Chunks] Channel closed")
					return
				}
				c.handleComposeChunks(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *UploadConsumer) handleComposeChunks(ctx context.Context, msg amqp.Delivery) {
	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer - Compose Chunks] Received message")

	var payload produce.ComposeChunksMessage
	if err := json.Unmarshal(msg.Body, &payload); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to unmarshal message")
		_ = msg.Nack(false, false)
		return
	}

	uploadID, err := uuid.Parse(payload.UploadID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Invalid upload ID")
		_ = msg.Nack(false, false)
		return
	}

	bucketID, err := uuid.Parse(payload.BucketID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Invalid bucket ID")
		_ = msg.Nack(false, false)
		return
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Invalid user ID")
		_ = msg.Nack(false, false)
		return
	}

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer - Compose Chunks] Processing upload %s for user %s", uploadID, userID)

	// Use a background context since this is a long-running operation
	// The HTTP request context is already canceled at this point
	bgCtx := context.Background()

	// 1. List all chunks
	chunks, err := c.infra.TempMinio.ListObjectsWithPrefix(bgCtx, payload.TempBucket, payload.TempPrefix)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to list chunks")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true) // Requeue
		return
	}

	if len(chunks) != payload.TotalChunks {
		c.infra.Logger.ErrorWithContextf(ctx, nil, "[Upload Consumer - Compose Chunks] Missing chunks: expected %d, found %d", payload.TotalChunks, len(chunks))
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, false)
		return
	}

	// 2. Sort chunks by key (chunk_00000, chunk_00001, ...)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Key < chunks[j].Key
	})

	sourcePaths := make([]string, len(chunks))
	for i, chunk := range chunks {
		sourcePaths[i] = chunk.Key
	}

	// 3. Compose chunks into single file
	ext := filepath.Ext(payload.FileName)
	if ext == "" {
		ext = ".bin"
	}

	composedKey := fmt.Sprintf("%scomposed%s", payload.TempPrefix, ext)

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer - Compose Chunks] Composing %d chunks into %s", len(chunks), composedKey)

	if err := c.infra.TempMinio.ComposeObject(bgCtx, payload.TempBucket, sourcePaths, composedKey); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to compose chunks")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true) // Requeue for retry
		return
	}

	// 4. Calculate SHA256 hash
	composedStream, composedSize, err := c.infra.TempMinio.GetObjectStream(bgCtx, payload.TempBucket, composedKey)
	if err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to get composed object for hashing")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true)
		return
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, composedStream); err != nil {
		composedStream.Close()
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to calculate file hash")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true)
		return
	}
	composedStream.Close()

	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// 5. Copy to final path with hash as name
	finalTempPath := fmt.Sprintf("%s%s", fileHash, ext)
	if err := c.infra.TempMinio.CopyObject(bgCtx, payload.TempBucket, composedKey, payload.TempBucket, finalTempPath); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to copy to final path")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true)
		return
	}

	// 6. Cleanup chunks and composed file (async)
	go func() {
		cleanupCtx := context.Background()
		for _, chunk := range chunks {
			_ = c.infra.TempMinio.DeleteObject(cleanupCtx, payload.TempBucket, chunk.Key)
		}
		_ = c.infra.TempMinio.DeleteObject(cleanupCtx, payload.TempBucket, composedKey)
	}()

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer - Compose Chunks] File composed with hash %s", fileHash)

	// 7. Update session with file hash
	if err := c.repository.UploadSessionRepo.UpdateFileHash(uploadID, fileHash); err != nil {
		c.infra.Logger.WarningWithContextf(ctx, "[Upload Consumer - Compose Chunks] Failed to update file hash: %v", err)
	}

	// 8. Create object record in database
	var targetFolder string
	if payload.CustomPath != "" {
		targetFolder = fmt.Sprintf("%s/%s", payload.CustomPath, fileHash)
	} else {
		targetFolder = fileHash
	}

	urlPart := fmt.Sprintf("%s%s", fileHash, ext)
	object := &entity.Object{
		ID:           uuid.New(),
		BucketID:     bucketID,
		ContentType:  payload.ContentType,
		OriginName:   payload.FileName,
		ParentPath:   payload.CustomPath,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Size:         composedSize,
		URL:          urlPart,
		FileHash:     fileHash,
	}

	if err := c.repository.ObjectRepo.Create(object); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to save object to database")
		c.updateSessionStatus(uploadID, entity.UploadStatusFailed)
		_ = msg.Nack(false, true)
		return
	}

	// 9. Publish ChunkedUploadMessage for further processing (move to final storage, etc.)
	uploadMsg := produce.ChunkedUploadMessage{
		UploadType:   getUploadType(ext),
		TempBucket:   payload.TempBucket,
		TempPath:     finalTempPath,
		TargetBucket: payload.BucketName,
		TargetFolder: targetFolder,
		OriginalName: payload.FileName,
		FileHash:     fileHash,
		FileSize:     composedSize,
		ChunkSize:    0,
		Metadata: map[string]string{
			"user_id":      payload.UserID,
			"bucket_id":    payload.BucketID,
			"content_type": payload.ContentType,
			"custom_path":  payload.CustomPath,
		},
	}

	if err := c.infra.Produce.UploadService.PublishChunkedUpload(bgCtx, uploadMsg); err != nil {
		c.infra.Logger.ErrorWithContextf(ctx, err, "[Upload Consumer - Compose Chunks] Failed to publish chunked upload message")
		// Don't fail the job - object is already created
		c.infra.Logger.WarningWithContextf(ctx, "[Upload Consumer - Compose Chunks] Object created but failed to queue for final processing")
	}

	// 10. Mark upload as completed
	c.updateSessionStatus(uploadID, entity.UploadStatusCompleted)

	c.infra.Logger.InfoWithContextf(ctx, "[Upload Consumer - Compose Chunks] Successfully completed upload %s, object %s created", uploadID, object.ID)

	// Acknowledge the message
	_ = msg.Ack(false)
}

func (c *UploadConsumer) updateSessionStatus(uploadID uuid.UUID, status entity.UploadStatus) {
	if err := c.repository.UploadSessionRepo.UpdateStatus(uploadID, status); err != nil {
		c.infra.Logger.WarningWithContextf(context.Background(), "[Upload Consumer] Failed to update session status: %v", err)
	}
}

// getUploadType determines the upload type based on file extension
func getUploadType(ext string) string {
	switch ext {
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "archive"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv":
		return "video"
	case ".mp3", ".wav", ".flac", ".aac":
		return "audio"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return "image"
	case ".exe", ".msi", ".dmg", ".deb", ".rpm":
		return "executable"
	default:
		return "file"
	}
}
