package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/tnqbao/gau-cloud-orchestrator/config"
	"github.com/tnqbao/gau-cloud-orchestrator/consumer/worker"
	infraPkg "github.com/tnqbao/gau-cloud-orchestrator/infra"
	"github.com/tnqbao/gau-cloud-orchestrator/repository"
)

func main() {
	err := godotenv.Load("../staging.env")
	if err != nil {
		log.Println("No .env file found, continuing with environment variables")
	}

	cfg := config.NewConfig()
	infra := infraPkg.InitInfra(cfg)
	repo := repository.InitRepository(infra)

	// Initialize context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start IAM Consumer
	iamConsumer := worker.NewIAMConsumer(infra.RabbitMQ.Channel, infra, repo)
	if err := iamConsumer.Start(ctx); err != nil {
		infra.Logger.ErrorWithContextf(ctx, err, "Failed to start IAM consumer: %v", err)
		log.Fatalf("Failed to start IAM consumer: %v", err)
	}

	// Start Bucket Consumer
	bucketConsumer := worker.NewBucketConsumer(infra.RabbitMQ.Channel, infra, repo)
	if err := bucketConsumer.Start(ctx); err != nil {
		infra.Logger.ErrorWithContextf(ctx, err, "Failed to start Bucket consumer: %v", err)
		log.Fatalf("Failed to start Bucket consumer: %v", err)
	}

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	infra.Logger.InfoWithContextf(ctx, "Shutting down consumer...")
	cancel() // Cancel context to stop consumers

	infra.Logger.InfoWithContextf(ctx, "Consumer exited properly")
}
