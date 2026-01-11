package controller

import (
	"github.com/tnqbao/gau-cloud-orchestrator/config"
	"github.com/tnqbao/gau-cloud-orchestrator/infra"
	"github.com/tnqbao/gau-cloud-orchestrator/repository"
)

type Controller struct {
	Config     *config.Config
	Infra      *infra.Infra
	Repository *repository.Repository
}

func NewController(config *config.Config, infra *infra.Infra, repo *repository.Repository) *Controller {
	if repo == nil {
		panic("Failed to initialize Repository")
	}
	return &Controller{
		Config:     config,
		Infra:      infra,
		Repository: repo,
	}
}
