package produce

import amqp "github.com/rabbitmq/amqp091-go"

type Produce struct {
	EmailService *EmailService
	IAMService   *IAMService
}

var produceInstance *Produce

func InitProduce(channel *amqp.Channel) *Produce {
	if produceInstance != nil {
		return produceInstance
	}

	emailService := InitEmailService(channel)
	if emailService == nil {
		panic("Failed to initialize Email service")
	}

	iamService := InitIAMService(channel)
	if iamService == nil {
		panic("Failed to initialize IAM service")
	}

	produceInstance = &Produce{
		EmailService: emailService,
		IAMService:   iamService,
	}

	return produceInstance
}

func GetProduce() *Produce {
	if produceInstance == nil {
		panic("Produce not initialized. Call InitProduce() first.")
	}
	return produceInstance
}
