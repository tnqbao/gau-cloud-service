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

	// ChunkCompleteQueue is sent to upload-service to trigger compose and move
	ChunkCompleteQueue      = "upload.chunk_complete"
	ChunkCompleteRoutingKey = "upload.chunk_complete"

	// ComposeCompletedQueue is received from upload-service after compose is done
	ComposeCompletedQueue      = "upload.compose_completed"
	ComposeCompletedRoutingKey = "upload.compose_completed"

	// ObjectDeleteQueue is for deleting single objects from storage
	ObjectDeleteQueue      = "object.delete"
	ObjectDeleteRoutingKey = "object.delete"

	// PathDeleteQueue is for deleting all objects in a path/folder from storage
	PathDeleteQueue      = "object.delete_path"
	PathDeleteRoutingKey = "object.delete_path"
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

// ChunkCompleteMessage is sent to upload-service when all chunks are uploaded
// Upload-service will compose chunks and move to final destination
type ChunkCompleteMessage struct {
	UploadID     string            `json:"upload_id"`     // Upload session ID
	BucketID     string            `json:"bucket_id"`     // Target bucket ID (cloud-orchestrator DB)
	BucketName   string            `json:"bucket_name"`   // Target bucket name in MinIO
	UserID       string            `json:"user_id"`       // User who initiated the upload
	TempBucket   string            `json:"temp_bucket"`   // Bucket containing chunks (pending)
	TempPrefix   string            `json:"temp_prefix"`   // Prefix for chunk objects (upload_id/)
	FileName     string            `json:"file_name"`     // Original file name
	FileSize     int64             `json:"file_size"`     // Expected total file size
	ContentType  string            `json:"content_type"`  // Content type
	CustomPath   string            `json:"custom_path"`   // Custom path in target bucket
	TotalChunks  int               `json:"total_chunks"`  // Total number of chunks
	TargetBucket string            `json:"target_bucket"` // Final destination bucket name
	TargetPath   string            `json:"target_path"`   // Final destination path (custom_path/)
	Metadata     map[string]string `json:"metadata"`      // Additional metadata
	Timestamp    int64             `json:"timestamp"`
}

// ComposeCompletedMessage is received from upload-service after compose is done
type ComposeCompletedMessage struct {
	UploadID    string `json:"upload_id"`    // Upload session ID
	BucketID    string `json:"bucket_id"`    // Target bucket ID
	UserID      string `json:"user_id"`      // User ID
	FileHash    string `json:"file_hash"`    // SHA256 hash of composed file
	FilePath    string `json:"file_path"`    // Final file path in bucket
	FileSize    int64  `json:"file_size"`    // Total file size
	ContentType string `json:"content_type"` // Content type
	FileName    string `json:"file_name"`    // Original file name
	CustomPath  string `json:"custom_path"`  // Custom path
	Success     bool   `json:"success"`      // Whether compose was successful
	Error       string `json:"error"`        // Error message if failed
	Timestamp   int64  `json:"timestamp"`
}

// DeleteObjectMessage is sent to consumer to delete a single object from storage
type DeleteObjectMessage struct {
	BucketName string `json:"bucket_name"` // MinIO bucket name
	ObjectPath string `json:"object_path"` // Object path in bucket (hash.ext format from URL field)
	UserID     string `json:"user_id"`     // User who triggered the delete
	Timestamp  int64  `json:"timestamp"`
}

// DeletePathMessage is sent to consumer to delete all objects in a path/folder from storage
type DeletePathMessage struct {
	BucketName string `json:"bucket_name"` // MinIO bucket name
	Path       string `json:"path"`        // Folder path to delete (prefix)
	UserID     string `json:"user_id"`     // User who triggered the delete
	Timestamp  int64  `json:"timestamp"`
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

	// Declare queue for legacy chunked upload
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

	// Declare ChunkComplete queue (sent to upload-service)
	_, err = channel.QueueDeclare(
		ChunkCompleteQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare ChunkComplete queue: " + err.Error())
	}

	// Bind ChunkComplete queue to exchange
	err = channel.QueueBind(
		ChunkCompleteQueue,
		ChunkCompleteRoutingKey,
		ChunkedUploadExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind ChunkComplete queue: " + err.Error())
	}

	// Declare ComposeCompleted queue (received from upload-service)
	_, err = channel.QueueDeclare(
		ComposeCompletedQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare ComposeCompleted queue: " + err.Error())
	}

	// Bind ComposeCompleted queue to exchange
	err = channel.QueueBind(
		ComposeCompletedQueue,
		ComposeCompletedRoutingKey,
		ChunkedUploadExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind ComposeCompleted queue: " + err.Error())
	}

	// Declare ObjectDelete queue
	_, err = channel.QueueDeclare(
		ObjectDeleteQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare ObjectDelete queue: " + err.Error())
	}

	// Bind ObjectDelete queue to exchange
	err = channel.QueueBind(
		ObjectDeleteQueue,
		ObjectDeleteRoutingKey,
		ChunkedUploadExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind ObjectDelete queue: " + err.Error())
	}

	// Declare PathDelete queue
	_, err = channel.QueueDeclare(
		PathDeleteQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare PathDelete queue: " + err.Error())
	}

	// Bind PathDelete queue to exchange
	err = channel.QueueBind(
		PathDeleteQueue,
		PathDeleteRoutingKey,
		ChunkedUploadExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind PathDelete queue: " + err.Error())
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

// PublishChunkComplete publishes a chunk complete message to upload-service
// This triggers upload-service to compose chunks and move to final destination
func (s *UploadProduceService) PublishChunkComplete(ctx context.Context, msg ChunkCompleteMessage) error {
	msg.Timestamp = time.Now().Unix()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		ChunkedUploadExchange,
		ChunkCompleteRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

// PublishDeleteObject publishes a message to delete a single object from storage
func (s *UploadProduceService) PublishDeleteObject(ctx context.Context, msg DeleteObjectMessage) error {
	msg.Timestamp = time.Now().Unix()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		ChunkedUploadExchange,
		ObjectDeleteRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

// PublishDeletePath publishes a message to delete all objects in a path from storage
func (s *UploadProduceService) PublishDeletePath(ctx context.Context, msg DeletePathMessage) error {
	msg.Timestamp = time.Now().Unix()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		ChunkedUploadExchange,
		PathDeleteRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}
