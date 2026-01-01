package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/tnqbao/gau-cloud-orchestrator/http/controller"
	middlewares "github.com/tnqbao/gau-cloud-orchestrator/http/middleware"
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
			aimRoutes.PUT("/:id", ctrl.UpdateIAMCredentials)
			//aimRoutes.PUT("/credentials/update", ctrl.UpdateIAMCredentials)
		}

		bucketRoutes := apiRoutes.Group("/buckets")
		{
			bucketRoutes.POST("/", ctrl.CreateBucket)
			bucketRoutes.GET("/", ctrl.ListBuckets)
			bucketRoutes.DELETE("/:id", ctrl.DeleteBucketByID)
			bucketRoutes.PUT("/:id/access", ctrl.UpdateBucketAccess)
			bucketRoutes.GET("/:id/access", ctrl.GetBucketAccess)

			// Object routes (nested under bucket)
			bucketRoutes.POST("/:id/objects", ctrl.UploadObject)
			bucketRoutes.GET("/:id/objects/*path", ctrl.ListObjectsByPath)
			bucketRoutes.DELETE("/:id/objects/:object_id", ctrl.DeleteObject)

			// Chunked upload routes (separate from /objects to avoid wildcard conflict)
			bucketRoutes.POST("/:id/chunked/init", ctrl.InitChunkedUpload)
			bucketRoutes.POST("/:id/chunked/chunk", ctrl.UploadChunk)
			bucketRoutes.POST("/:id/chunked/complete", ctrl.CompleteChunkedUpload)
			bucketRoutes.GET("/:id/chunked/:upload_id/progress", ctrl.GetUploadProgress)
			bucketRoutes.DELETE("/:id/chunked/:upload_id", ctrl.AbortChunkedUpload)
		}

	}
	return r
}
