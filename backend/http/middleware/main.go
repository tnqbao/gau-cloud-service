package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/tnqbao/gau-cloud-orchestrator/http/controller"
)

type Middlewares struct {
	CORSMiddleware gin.HandlerFunc
	AuthMiddleware gin.HandlerFunc
}

func NewMiddlewares(ctrl *controller.Controller) (*Middlewares, error) {
	cors := CORSMiddleware(ctrl.Config.EnvConfig)
	auth := AuthMiddleware(ctrl.Infra.AuthorizationService, ctrl.Config.EnvConfig)

	return &Middlewares{
		CORSMiddleware: cors,
		AuthMiddleware: auth,
	}, nil
}
