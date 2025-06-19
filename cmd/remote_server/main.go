package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	r := gin.Default()

	r.POST("/calculate", func(c *gin.Context) {
		// Получаем параметры из запроса (JSON)
		var input struct {
			Operation string  `json:"operation"`
			X         float64 `json:"x"`
			Y         float64 `json:"y"`
		}

		if err := c.BindJSON(&input); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		op := input.Operation
		x := input.X
		y := input.Y

		var result float64
		switch op {
		case "add":
			result = x + y
		case "subtract":
			result = x - y
		case "multiply":
			result = x * y
		case "divide":
			if y == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cannot divide by zero"})
				return
			}
			result = x / y
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operation"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"result": result,
		})
	})

	// Запуск сервера
	r.Run(":9090")
}
