package main

import (
	"context"
	"fmt"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"
)

const systemPrompt = `Ты - ассистент с доступом к инструментам. Строго соблюдай правила:

1. Для математических операций ВСЕГДА используй инструмент calculate
2. Для приветствий ВСЕГДА используй инструмент hello_world
3. Формат ответа ДОЛЖЕН быть:

Action: имя_инструмента
Action Input: {"параметр":"значение"}

ИЛИ

Final Answer: текст ответа

Примеры:
---
User: Привет, Анна
Assistant:
Action: hello_world
Action Input: {"name":"Анна"}
---
User: Сколько будет 5+3?
Assistant:
Action: calculate
Action Input: {"operation":"add","x":5,"y":3}
---
User: Как дела?
Assistant:
Final Answer: У меня все хорошо!`

func main() {
	ctx := context.Background()

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

	// Получаем инструменты с сервера
	agentTools, err := getTools(ctx, mcpClient)
	if err != nil {
		log.Fatal(err)
	}

	promptTemplate := prompts.NewPromptTemplate(
		`{{.system_prompt}}
		Текущая дата: {{.today}}
		Запрос: {{.input}}`,
		[]string{"system_prompt", "today", "input"},
	)

	// 4. Настраиваем агента с улучшенными промптами
	agent := agents.NewOneShotAgent(
		llm,
		agentTools,
		agents.WithMemory(memory.NewConversationBuffer()),
		agents.WithPrompt(promptTemplate), // Передаем нашу цепочку
	)

	// 5. Примеры запросов для тестирования
	testQueries := []struct {
		text string
	}{
		{"Поздоровайся с Анной"},
		{"Сколько будет 7 * 8 ?"},
		{"Раздели 100 / 4"},
	}

	for _, query := range testQueries {
		fmt.Printf("\n=== Тест: %q ===\n", query.text)

		// 6. Выполняем запрос через агента
		actions, finish, err := agent.Plan(ctx, []schema.AgentStep{}, map[string]string{
			"system_prompt": systemPrompt,
			"input":         query.text,
		})

		if err != nil {
			log.Printf("Ошибка: %v", err)
			continue
		}

		// 7. Анализируем результат
		if finish != nil {
			fmt.Printf("⚠ Агент ответил напрямую: %s\n", finish.ReturnValues["output"])
			continue
		}

		if len(actions) > 0 {
			agentSteps := []schema.AgentStep{}
			for _, action := range actions {
				fmt.Printf("✅ Агент выбрал инструмент: %s\n", action.Tool)

				// 8. Выполняем инструмент
				result, err := executeTool(ctx, agentTools, action)
				if err != nil {
					log.Printf("Ошибка инструмента: %v", err)
					continue
				}

				fmt.Printf("🛠 Результат инструмента: %s\n", result)

				// 9. Передаем результат обратно агенту
				agentSteps := append(agentSteps, schema.AgentStep{Action: action, Observation: result})

				_, finish, err := agent.Plan(ctx, agentSteps, map[string]string{
					"system_prompt": systemPrompt,
					"input":         query.text,
				})

				if err != nil {
					fmt.Printf("Ошибка агента после использования инструмента: %v", err)
					continue
				}

				if finish != nil {
					fmt.Printf("📝 Финальный ответ: %s\n", finish.ReturnValues["output"])
					continue
				}
			}

		} else {
			fmt.Println("❌ Агент не выбрал ни ответа, ни инструмента")
		}
	}
}

func executeTool(ctx context.Context, tools []tools.Tool, action schema.AgentAction) (string, error) {
	tool := findToolByName(tools, action.Tool)
	if tool == nil {
		return "", fmt.Errorf("инструмент %s не найден", action.Tool)
	}
	return tool.Call(ctx, action.ToolInput)
}

func findToolByName(tools []tools.Tool, name string) tools.Tool {
	for _, tool := range tools {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
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

func getTools(ctx context.Context, c *client.Client) ([]tools.Tool, error) {
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения инструментов: %v", err)
	}

	var langchainTools []tools.Tool
	for _, tool := range toolsResult.Tools {
		fmt.Printf("🛠 Зарегистрирован инструмент: %s\n   Описание: %s\n", tool.Name, tool.Description)
		langchainTools = append(langchainTools, NewMCToolAdapter(tool, c))
	}

	return langchainTools, nil
}
