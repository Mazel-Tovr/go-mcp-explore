package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	swagFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "go-mcp/docs"
	"go-mcp/internal/tool"
	"net/http"
	"os"
)

// @title Calculator API
// @version 1.0
// @description Простой калькулятор с операциями add, subtract, multiply, divide.
// @host localhost:9090
// @BasePath /
func main() {
	r := gin.Default()

	// Swagger route
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swagFiles.Handler))

	r.POST("/calculate", calculateHandler)

	r.GET("/endpoints", func(c *gin.Context) {
		file, err := os.ReadFile("resources/endoints.json")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read endpoints.json"})
			return
		}
		response := tool.DiscoveryServiceResponse{}
		err = json.Unmarshal(file, &response)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse endpoints.json"})
			return
		}

		c.JSON(http.StatusOK, response)
	})

	// Запуск сервера
	err := r.Run(":9090")
	if err != nil {
		return
	}
}

// Calculate godoc
// @Summary Выполнить арифметическую операцию
// @Description Выполняет сложение, вычитание, умножение или деление двух чисел
// @Tags calculator
// @Accept json
// @Produce json
// @Param request body main.calculateInput true "Параметры запроса"
// @Success 200 {object} map[string]float64
// @Failure 400 {object} map[string]string
// @Router /calculate [post]
func calculateHandler(c *gin.Context) {
	fmt.Println("calculateHandler invoked")
	var input calculateInput

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
}

type calculateInput struct {
	Operation string  `json:"operation"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}
