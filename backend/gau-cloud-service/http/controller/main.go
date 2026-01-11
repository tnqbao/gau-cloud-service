package controller

import (
	"github.com/tnqbao/gau-cloud-service/config"
	"github.com/tnqbao/gau-cloud-service/infra"
	"github.com/tnqbao/gau-cloud-service/repository"
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
