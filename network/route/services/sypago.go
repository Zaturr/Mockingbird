package services

import (
	"Mockingbird/network/handler"

	"github.com/gin-gonic/gin"
)

func SetupSypagoRoutes(router *gin.Engine) {
	sypago := router.Group("/api/sypago")
	{
		sypago.POST("/payments", handler.CreatePayment)
		sypago.GET("/payments/:id", handler.GetPayment)
		sypago.POST("/transactions", handler.CreateTransaction)
	}
}
