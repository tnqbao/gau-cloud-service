package produce

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailMessage struct {
	Type          string `json:"type"`
	Recipient     string `json:"recipient"`
	RecipientName string `json:"recipientName,omitempty"`
	Content       string `json:"content"`
	ActionUrl     string `json:"actionUrl,omitempty"`
}

type EmailService struct {
	channel *amqp.Channel
}

func InitEmailService(channel *amqp.Channel) *EmailService {
	return &EmailService{
		channel: channel,
	}
}

func (s *EmailService) SendEmailConfirmation(ctx context.Context, email, recipientName, content, actionUrl string) error {
	message := EmailMessage{
		Type:          "confirmation",
		Recipient:     email,
		RecipientName: recipientName,
		Content:       content,
		ActionUrl:     actionUrl,
	}

	return s.publishEmail(ctx, "email.confirmation", message)
}

func (s *EmailService) SendEmailNotification(ctx context.Context, email, recipientName, content, actionUrl string) error {
	message := EmailMessage{
		Type:          "notification",
		Recipient:     email,
		RecipientName: recipientName,
		Content:       content,
		ActionUrl:     actionUrl,
	}

	return s.publishEmail(ctx, "email.notification", message)
}

func (s *EmailService) SendEmailWarning(ctx context.Context, email, recipientName, content, actionUrl string) error {
	message := EmailMessage{
		Type:          "warning",
		Recipient:     email,
		RecipientName: recipientName,
		Content:       content,
		ActionUrl:     actionUrl,
	}

	return s.publishEmail(ctx, "email.warning", message)
}

func (s *EmailService) publishEmail(ctx context.Context, routingKey string, message EmailMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal email message: %w", err)
	}

	err = s.channel.PublishWithContext(
		ctx,
		"email_exchange", // exchange
		routingKey,       // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish email message: %w", err)
	}

	return nil
}
