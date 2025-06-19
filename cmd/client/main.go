package main

import (
	"context"
	"fmt"
	"go-mcp/internal/agent"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/llms/openai"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Инициализация LLM (OpenAI)
	llm, err := openai.New(
		openai.WithBaseURL("http://localhost:8090"),
		openai.WithToken("some-token"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Создаем MCP клиент
	mcpClient, err := createClient()
	if err != nil {
		log.Fatal(err)
	}

	// Инициализация MCP клиента
	if err := initMCPClient(ctx, mcpClient); err != nil {
		log.Fatal(err)
	}

	aiAgent, err := agent.NewAgent(mcpClient, llm)
	if err != nil {
		log.Fatal(err)
	}

	// Обработчик сигналов для завершения работы
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nПолучен сигнал завершения работы. Завершаем...")
		cancel()
		os.Exit(0)
	}()

	// Основной цикл интерактивного диалога
	for {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Print("\nВведите ваш запрос (или 'exit' для выхода): ")
			var userInput string
			if _, err := fmt.Scanln(&userInput); err != nil {
				log.Printf("Ошибка ввода: %v", err)
				continue
			}

			if userInput == "exit" {
				fmt.Println("Завершение работы...")
				return
			}

			// Выполняем запрос через агента
			if err := aiAgent.ExecuteQuery(userInput); err != nil {
				fmt.Printf("Ошибка выполнения запроса: %v", err)
				return
			}
		}
	}
}

func createClient() (*client.Client, error) {
	t, err := transport.NewStreamableHTTP("http://localhost:8080/mcp")
	if err != nil {
		return nil, fmt.Errorf("ошибка создания транспорта: %v", err)
	}
	return client.NewClient(t), nil
}

func initMCPClient(ctx context.Context, c *client.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := c.Start(ctx); err != nil {
		return fmt.Errorf("ошибка запуска клиента: %v", err)
	}

	c.OnNotification(func(n mcp.JSONRPCNotification) {
		log.Printf("Уведомление: %s", n.Method)
	})

	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "MCP Agent Client",
				Version: "1.1.0",
			},
		},
	}

	if _, err := c.Initialize(ctx, initRequest); err != nil {
		return fmt.Errorf("ошибка инициализации: %v", err)
	}

	return nil
}
