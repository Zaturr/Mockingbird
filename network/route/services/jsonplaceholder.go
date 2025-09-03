package services

import (
	"Mockingbird/network/handler"

	"github.com/gin-gonic/gin"
)

func SetupJsonplaceholderRoutes(router *gin.Engine) {
	jsonplaceholder := router.Group("/api/jsonplaceholder")
	{
		// Rutas POST existentes
		jsonplaceholder.POST("/posts", handler.CreatePost)
		jsonplaceholder.POST("/users", handler.CreateUser)
		jsonplaceholder.POST("/comments", handler.CreateComment)

		// Nuevas rutas GET con parámetros por URL para pruebas
		jsonplaceholder.GET("/posts/:id", handler.GetPostByID)
		jsonplaceholder.GET("/users/:id", handler.GetUserByID)
		jsonplaceholder.GET("/comments/:id", handler.GetCommentByID)
	}
}
