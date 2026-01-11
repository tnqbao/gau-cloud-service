package produce

import amqp "github.com/rabbitmq/amqp091-go"

type Produce struct {
	EmailService  *EmailService
	IAMService    *IAMService
	BucketService *BucketService
	UploadService *UploadProduceService
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

	bucketService := InitBucketService(channel)
	if bucketService == nil {
		panic("Failed to initialize Bucket service")
	}

	uploadService := InitUploadProduceService(channel)
	if uploadService == nil {
		panic("Failed to initialize Upload produce service")
	}

	produceInstance = &Produce{
		EmailService:  emailService,
		IAMService:    iamService,
		BucketService: bucketService,
		UploadService: uploadService,
	}

	return produceInstance
}

func GetProduce() *Produce {
	if produceInstance == nil {
		panic("Produce not initialized. Call InitProduce() first.")
	}
	return produceInstance
}
