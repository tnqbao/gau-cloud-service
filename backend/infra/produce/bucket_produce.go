package produce

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	BucketUpdatePolicyQueue      = "bucket.update.policy"
	BucketUpdatePolicyExchange   = "bucket.exchange"
	BucketUpdatePolicyRoutingKey = "bucket.update.policy"
)

type BucketService struct {
	channel *amqp.Channel
}

type UpdateBucketPolicyMessage struct {
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
		BucketUpdatePolicyExchange,
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

	// Declare update policy queue
	_, err = channel.QueueDeclare(
		BucketUpdatePolicyQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare Bucket update policy queue: " + err.Error())
	}

	// Bind update policy queue to exchange
	err = channel.QueueBind(
		BucketUpdatePolicyQueue,
		BucketUpdatePolicyRoutingKey,
		BucketUpdatePolicyExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind Bucket update policy queue: " + err.Error())
	}

	return service
}

func (s *BucketService) PublishUpdateBucketPolicy(ctx context.Context, userID, bucketName string) error {
	message := UpdateBucketPolicyMessage{
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
		BucketUpdatePolicyExchange,
		BucketUpdatePolicyRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}
