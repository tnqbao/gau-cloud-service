package infra

import (
	"github.com/tnqbao/gau-cloud-service/config"
	"github.com/tnqbao/gau-cloud-service/infra/produce"
)

type Infra struct {
	Redis                *RedisClient
	Postgres             *PostgresClient
	Logger               *LoggerClient
	RabbitMQ             *RabbitMQClient
	AuthorizationService *AuthorizationService
	UploadService        *UploadService
	Produce              *produce.Produce
	Minio                *MinioClient
}

var infraInstance *Infra

func InitInfra(cfg *config.Config) *Infra {
	if infraInstance != nil {
		return infraInstance
	}

	redis := InitRedisClient(cfg.EnvConfig)
	if redis == nil {
		panic("Failed to initialize Redis service")
	}

	postgres := InitPostgresClient(cfg.EnvConfig)
	if postgres == nil {
		panic("Failed to initialize Postgres service")
	}

	logger := InitLoggerClient(cfg.EnvConfig)
	if logger == nil {
		panic("Failed to initialize Logger service")
	}

	rabbitMQ := InitRabbitMQClient(cfg.EnvConfig)
	if rabbitMQ == nil {
		panic("Failed to initialize RabbitMQ service")
	}

	authorizationService := InitAuthorizationService(cfg.EnvConfig)
	if authorizationService == nil {
		panic("Failed to initialize Authorization service")
	}

	uploadService := InitUploadService(cfg.EnvConfig)
	if uploadService == nil {
		panic("Failed to initialize Upload service")
	}

	produceService := produce.InitProduce(rabbitMQ.Channel)
	if produceService == nil {
		panic("Failed to initialize Produce service")
	}

	minio := InitMinioClient(cfg.EnvConfig)
	if minio == nil {
		panic("Failed to initialize MinIO service")
	}

	infraInstance = &Infra{
		Redis:                redis,
		Postgres:             postgres,
		Logger:               logger,
		RabbitMQ:             rabbitMQ,
		AuthorizationService: authorizationService,
		UploadService:        uploadService,
		Produce:              produceService,
		Minio:                minio,
	}

	return infraInstance
}

func GetClient() *Infra {
	if infraInstance == nil {
		panic("Infra not initialized. Call InitInfra() first.")
	}
	return infraInstance
}
