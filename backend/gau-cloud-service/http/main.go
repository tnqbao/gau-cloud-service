package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/tnqbao/gau-cloud-service/config"
	"github.com/tnqbao/gau-cloud-service/http/controller"
	"github.com/tnqbao/gau-cloud-service/http/route"
	infraPkg "github.com/tnqbao/gau-cloud-service/infra"
	"github.com/tnqbao/gau-cloud-service/repository"
)

func main() {
	err := godotenv.Load("staging.env")
	if err != nil {
		log.Println("No .env file found, continuing with environment variables")
	}

	cfg := config.NewConfig()
	infra := infraPkg.InitInfra(cfg)
	repo := repository.InitRepository(infra)

	ctrl := controller.NewController(cfg, infra, repo)

	router := routes.SetupRouter(ctrl)

	log.Println("HTTP Server started on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
