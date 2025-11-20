package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/tnqbao/gau-cloud-orchestrator/controller"
	"github.com/tnqbao/gau-cloud-orchestrator/middleware"
)

func SetupRouter(ctrl *controller.Controller) *gin.Engine {
	r := gin.Default()
	middles, err := middlewares.NewMiddlewares(ctrl)
	if err != nil {
		panic(err)
	}

	apiRoutes := r.Group("/api/v1/cloud")
	{
		apiRoutes.Use(middles.AuthMiddleware)

		aimRoutes := apiRoutes.Group("/iam")
		{
			aimRoutes.POST("/", ctrl.CreateIAM)
			aimRoutes.GET("/", ctrl.ListIAMs)
			aimRoutes.DELETE("/:id", ctrl.DeleteIAMByID)
			aimRoutes.PUT("/:id", ctrl.UpdateIAMByID)
		}

	}
	return r
}
