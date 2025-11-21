package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/tnqbao/gau-cloud-orchestrator/config"
	"github.com/tnqbao/gau-cloud-orchestrator/http/controller"
	"github.com/tnqbao/gau-cloud-orchestrator/http/route"
	"github.com/tnqbao/gau-cloud-orchestrator/infra"
	"github.com/tnqbao/gau-cloud-orchestrator/repository"
)

func main() {
	err := godotenv.Load("staging.env")
	if err != nil {
		log.Println("No .env file found, continuing with environment variables")
	}

	cfg := config.NewConfig()
	infra := infra.InitInfra(cfg)
	repo := repository.InitRepository(infra)

	ctrl := controller.NewController(cfg, infra, repo)

	router := routes.SetupRouter(ctrl)
	router.Run(":8080")
}
