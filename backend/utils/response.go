package utils

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func JSON200(c *gin.Context, data gin.H) {
	data["status"] = 200
	c.JSON(200, data)
}

func JSON202(c *gin.Context, data gin.H) {
	data["status"] = 202
	c.JSON(202, data)
}

func JSON500(c *gin.Context, err string) {
	fmt.Print("Error: ", err, "\n")
	c.JSON(500, gin.H{
		"error":  "Internal Server Error",
		"status": 500,
	})
}

func JSON400(c *gin.Context, err string) {
	c.JSON(400, gin.H{
		"error":  err,
		"status": 400,
	})
}

func JSON401(c *gin.Context, err string) {
	c.JSON(401, gin.H{
		"error":  err,
		"status": 401,
	})
}

func JSON403(c *gin.Context, err string) {
	c.JSON(403, gin.H{
		"error":  err,
		"status": 403,
	})
}

func JSON409(c *gin.Context, err string) {
	c.JSON(409, gin.H{
		"error":  err,
		"status": 409,
	})
}

func JSON404(c *gin.Context, err string) {
	c.JSON(404, gin.H{
		"error":  err,
		"status": 404,
	})
}

func JSON413(c *gin.Context, data gin.H) {
	data["status"] = 413
	c.JSON(413, data)
}
