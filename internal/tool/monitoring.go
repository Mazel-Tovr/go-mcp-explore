package tool

import (
	"encoding/json"
	"fmt"
	"github.com/ckanthony/openapi-mcp/pkg/config"
	"github.com/ckanthony/openapi-mcp/pkg/parser"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Monitor struct {
	server        *server.MCPServer
	toolDiscovery DiscoveryService
}

func NewMonitor(server *server.MCPServer, toolDiscovery DiscoveryService) *Monitor {
	m := &Monitor{server: server, toolDiscovery: toolDiscovery}
	toolDiscovery.RegisterEndpointsChangedCallback(m.OnEndpointUpdates)
	return m
}

func (m *Monitor) OnEndpointUpdates(endpoints []string) {
	conf := &config.Config{}
	// Хранение уже зарегистрированных инструментов для предотвращения дубляжа
	registeredTools := make(map[string]struct{})

	for _, endpoint := range endpoints {
		swagger, version, err := parser.LoadSwagger(endpoint)
		if err != nil {
			fmt.Printf("Failed to load swagger from %s: %v\n", endpoint, err)
			continue
		}

		toolSet, err := parser.GenerateToolSet(swagger, version, conf)
		if err != nil {
			fmt.Printf("Failed to generate tool set for %s: %v\n", endpoint, err)
			continue
		}

		for _, tool := range toolSet.Tools {
			// Проверка, не зарегистрирован ли инструмент уже
			if _, exists := registeredTools[tool.Name]; exists {
				fmt.Printf("Tool already registered tool: %s\n", tool.Name)
				fmt.Printf("Trying to register it again: %s\n", tool.Name)
				m.server.DeleteTools(tool.Name)
			}
			// Создаем новый Tool для mcp-go
			marshal, err := json.Marshal(tool.InputSchema)
			if err != nil {
				fmt.Printf("Failed to marshal tool input for %s: %v\n", tool.Name, err)
				continue
			}
			mcpTool := mcp.NewToolWithRawSchema(tool.Name, tool.Description, marshal)
			// Регистрируем инструмент на сервере
			m.server.AddTool(mcpTool, NewGenericToolHandler(toolSet, tool.Name))
			// Отмечаем, что инструмент зарегистрирован
			registeredTools[tool.Name] = struct{}{}
			fmt.Printf("Registered tool: %s  %s \n", tool.Name, tool.Description)

		}
	}
}
