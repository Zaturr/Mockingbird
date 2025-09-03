package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Payment struct {
	ID       int     `json:"id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
}

type Transaction struct {
	ID        int     `json:"id"`
	PaymentID int     `json:"paymentId"`
	Amount    float64 `json:"amount"`
	Type      string  `json:"type"`
}

func CreatePayment(c *gin.Context) {
	var payment Payment
	if err := c.ShouldBindJSON(&payment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Pago creado exitosamente",
		"payment": payment,
	})
}

func GetPayment(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Pago obtenido exitosamente",
		"payment": Payment{ID: 1, Amount: 100.00, Currency: "USD", Status: "completed"},
	})
}

func CreateTransaction(c *gin.Context) {
	var transaction Transaction
	if err := c.ShouldBindJSON(&transaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Transacción creada exitosamente",
		"transaction": transaction,
	})
}
