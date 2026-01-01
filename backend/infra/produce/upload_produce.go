package produce

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ChunkedUploadQueue      = "upload.chunked"
	ChunkedUploadExchange   = "upload.exchange"
	ChunkedUploadRoutingKey = "upload.chunked"
)

// ChunkedUploadMessage represents the message structure for chunked uploads
type ChunkedUploadMessage struct {
	UploadType   string            `json:"upload_type"`   // e.g., "zip", "video", "archive"
	TempBucket   string            `json:"temp_bucket"`   // Bucket in temp MinIO
	TempPath     string            `json:"temp_path"`     // Path in temp MinIO
	TargetBucket string            `json:"target_bucket"` // Target bucket in main MinIO
	TargetFolder string            `json:"target_folder"` // Target folder for chunks
	OriginalName string            `json:"original_name"` // Original file name before hashing
	FileHash     string            `json:"file_hash"`     // Hash of the file
	FileSize     int64             `json:"file_size"`     // Total file size in bytes
	ChunkSize    int64             `json:"chunk_size"`    // Desired chunk size (0 = use default)
	Metadata     map[string]string `json:"metadata"`      // Additional metadata (user_id, upload_id, etc.)
	Timestamp    int64             `json:"timestamp"`
}

// UploadProduceService handles publishing messages for upload processing
type UploadProduceService struct {
	channel *amqp.Channel
}

// InitUploadProduceService initializes the upload produce service
func InitUploadProduceService(channel *amqp.Channel) *UploadProduceService {
	service := &UploadProduceService{
		channel: channel,
	}

	// Declare exchange
	err := channel.ExchangeDeclare(
		ChunkedUploadExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to declare Upload exchange: " + err.Error())
	}

	// Declare queue
	_, err = channel.QueueDeclare(
		ChunkedUploadQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare Chunked Upload queue: " + err.Error())
	}

	// Bind queue to exchange
	err = channel.QueueBind(
		ChunkedUploadQueue,
		ChunkedUploadRoutingKey,
		ChunkedUploadExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind Chunked Upload queue: " + err.Error())
	}

	return service
}

// PublishChunkedUpload publishes a chunked upload message to the queue
func (s *UploadProduceService) PublishChunkedUpload(ctx context.Context, msg ChunkedUploadMessage) error {
	msg.Timestamp = time.Now().Unix()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		ChunkedUploadExchange,
		ChunkedUploadRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}
