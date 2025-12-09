package produce

import amqp "github.com/rabbitmq/amqp091-go"

type Produce struct {
	EmailService  *EmailService
	BucketService *BucketService
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

	bucketService := InitBucketService(channel)
	if bucketService == nil {
		panic("Failed to initialize Bucket service")
	}

	produceInstance = &Produce{
		EmailService:  emailService,
		BucketService: bucketService,
	}

	return produceInstance
}

func GetProduce() *Produce {
	if produceInstance == nil {
		panic("Produce not initialized. Call InitProduce() first.")
	}
	return produceInstance
}
