package agent

import (
	"context"
	"fmt"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"
	"log"
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

var promptTemplate = agents.WithPrompt(prompts.NewPromptTemplate(
	`{{.system_prompt}}
		Текущая дата: {{.today}}
		Запрос: {{.input}}`,
	[]string{"system_prompt", "today", "input"},
))

type Agent struct {
	c     *client.Client
	llm   *openai.LLM
	agent *agents.OneShotZeroAgent
}

func NewAgent(c *client.Client, llm *openai.LLM) (*Agent, error) {
	// Получаем инструменты с сервера
	agentTools, err := getTools(c)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения инструментов: %v", err)
	}

	// 4. Настраиваем агента с улучшенными промптами
	agent := agents.NewOneShotAgent(
		llm,
		agentTools,
		agents.WithMemory(memory.NewConversationBuffer()),
		promptTemplate, // Передаем нашу цепочку
	)
	a := &Agent{c: c, llm: llm, agent: agent}
	c.OnNotification(a.OnEventFromMCPServer)
	return a, nil
}

func (a *Agent) OnEventFromMCPServer(notification mcp.JSONRPCNotification) {
	if notification.Method == mcp.MethodNotificationToolsListChanged {
		err := a.UpdateTools()
		if err != nil {
			log.Printf("Ошибка обновления инструментов агента: %v", err)
		}
	} else {
		fmt.Printf("Агент игнарирует событие от сервера: %s\n", notification.Method)
	}
}

func (a *Agent) UpdateTools() error {
	agentTools, err := getTools(a.c)
	if err != nil {
		return fmt.Errorf("ошибка получения инструментов: %v", err)
	}
	fmt.Printf("Пытаемся обновить агента с новым набором инструментов %v\n", agentTools)

	agent := agents.NewOneShotAgent(
		a.llm,
		agentTools,
		agents.WithMemory(memory.NewConversationBuffer()),
		promptTemplate, // Передаем нашу цепочку
	)
	a.agent = agent
	return nil
}

func (a *Agent) ExecuteQuery(input string) error {
	ctx := context.Background()

	fmt.Printf("User input: %s\n", input)

	// 6. Выполняем запрос через агента
	actions, finish, err := a.agent.Plan(ctx, []schema.AgentStep{}, map[string]string{
		"system_prompt": systemPrompt,
		"input":         input,
	})

	if err != nil {
		return fmt.Errorf("Ошибка агента: %v", err)
	}

	// 7. Анализируем результат
	if finish != nil {
		fmt.Printf("⚠ Агент ответил напрямую: %s\n", finish.ReturnValues["output"])
		return nil
	}

	if len(actions) > 0 {
		agentSteps := []schema.AgentStep{}
		for _, action := range actions {
			fmt.Printf("✅ Агент выбрал инструмент: %s\n", action.Tool)

			// 8. Выполняем инструмент
			result, err := a.executeTool(ctx, action)
			if err != nil {
				log.Printf("Ошибка инструмента: %v", err)
				continue
			}

			fmt.Printf("🛠 Результат инструмента: %s\n", result)

			// 9. Передаем результат обратно агенту
			agentSteps := append(agentSteps, schema.AgentStep{Action: action, Observation: result})

			_, finish, err := a.agent.Plan(ctx, agentSteps, map[string]string{
				"system_prompt": systemPrompt,
				"input":         input,
			})

			if err != nil {
				fmt.Printf("Ошибка агента после использования инструмента: %v", err)
				continue
			}

			if finish != nil {
				fmt.Printf("📝 Финальный ответ: %s\n", finish.ReturnValues["output"])
				continue
			} else {
				fmt.Printf("Агента не дал филнальног ответа")
			}
		}
	} else {
		fmt.Println("❌ Агент не выбрал ни ответа, ни инструмента")
	}
	return nil
}

func getTools(c *client.Client) ([]tools.Tool, error) {
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := c.ListTools(context.Background(), toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения инструментов: %v", err)
	}

	var agentTools []tools.Tool
	for _, tool := range toolsResult.Tools {
		fmt.Printf("🛠 Зарегистрирован инструмент: %s\n   Описание: %s\n", tool.Name, tool.Description)
		agentTools = append(agentTools, NewMCToolAdapter(tool, c))
	}

	return agentTools, nil
}

func (a *Agent) executeTool(ctx context.Context, action schema.AgentAction) (string, error) {
	tool := findToolByName(a.agent.Tools, action.Tool)
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
