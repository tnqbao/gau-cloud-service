package produce

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	IAMUpdateCredentialsQueue      = "iam.update.credentials"
	IAMUpdateCredentialsExchange   = "iam.exchange"
	IAMUpdateCredentialsRoutingKey = "iam.update.credentials"

	IAMUpdatePolicyQueue      = "iam.update.policy"
	IAMUpdatePolicyRoutingKey = "iam.update.policy"
)

type IAMService struct {
	channel *amqp.Channel
}

type UpdateIAMCredentialsMessage struct {
	IAMID     string `json:"iam_id"`
	UserID    string `json:"user_id"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Timestamp int64  `json:"timestamp"`
}

type UpdateIAMPolicyMessage struct {
	IAMID         string `json:"iam_id"`
	OldPolicyName string `json:"old_policy_name"`
	NewPolicyName string `json:"new_policy_name"`
	PolicyJSON    []byte `json:"policy_json"`
	Timestamp     int64  `json:"timestamp"`
}

func InitIAMService(channel *amqp.Channel) *IAMService {
	service := &IAMService{
		channel: channel,
	}

	// Declare exchange
	err := channel.ExchangeDeclare(
		IAMUpdateCredentialsExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to declare IAM exchange: " + err.Error())
	}

	// Declare update credentials queue
	_, err = channel.QueueDeclare(
		IAMUpdateCredentialsQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare IAM update credentials queue: " + err.Error())
	}

	// Bind update credentials queue to exchange
	err = channel.QueueBind(
		IAMUpdateCredentialsQueue,
		IAMUpdateCredentialsRoutingKey,
		IAMUpdateCredentialsExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind IAM update credentials queue: " + err.Error())
	}

	// Declare update policy queue
	_, err = channel.QueueDeclare(
		IAMUpdatePolicyQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		panic("Failed to declare IAM update policy queue: " + err.Error())
	}

	// Bind update policy queue to exchange
	err = channel.QueueBind(
		IAMUpdatePolicyQueue,
		IAMUpdatePolicyRoutingKey,
		IAMUpdateCredentialsExchange,
		false,
		nil,
	)
	if err != nil {
		panic("Failed to bind IAM update policy queue: " + err.Error())
	}

	return service
}

func (s *IAMService) PublishUpdateCredentials(ctx context.Context, msg UpdateIAMCredentialsMessage) error {
	msg.Timestamp = time.Now().Unix()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		IAMUpdateCredentialsExchange,
		IAMUpdateCredentialsRoutingKey,
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

func (s *IAMService) PublishUpdatePolicy(ctx context.Context, msg UpdateIAMPolicyMessage) error {
	msg.Timestamp = time.Now().Unix()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.channel.PublishWithContext(
		ctx,
		IAMUpdateCredentialsExchange,
		IAMUpdatePolicyRoutingKey,
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
