package produce

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	BucketExchange         = "bucket.exchange"
	BucketDeleteQueue      = "bucket.delete"
	BucketDeleteRoutingKey = "bucket.delete"
)

type BucketService struct {
	channel *amqp.Channel
}

type DeleteBucketMessage struct {
	UserID     string `json:"user_id"`
	BucketName string `json:"bucket_name"`
	Timestamp  int64  `json:"timestamp"`
}

func InitBucketService(channel *amqp.Channel) *BucketService {
	service := &BucketService{
		channel: channel,
	}

	// Declare exchange
	err := channel.ExchangeDeclare(
		BucketExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to declare Bucket exchange: " + err.Error())
	}

	// Declare delete bucket queue
	_, err = channel.QueueDeclare(
		BucketDeleteQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare Bucket delete queue: " + err.Error())
	}

	// Bind delete bucket queue to exchange
	err = channel.QueueBind(
		BucketDeleteQueue,
		BucketDeleteRoutingKey,
		BucketExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind Bucket delete queue: " + err.Error())
	}

	return service
}

func (s *BucketService) PublishDeleteBucket(ctx context.Context, userID, bucketName string) error {
	message := DeleteBucketMessage{
		UserID:     userID,
		BucketName: bucketName,
		Timestamp:  time.Now().Unix(),
	}

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		BucketExchange,
		BucketDeleteRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}
