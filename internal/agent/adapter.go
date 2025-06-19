package agent

import (
	"context"
	"encoding/json"
	"fmt"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp" // или где у тебя определен mcp.Tool
)

// MCToolAdapter оборачивает mcp.Tool и реализует langchaingo/tools.Tool
type MCToolAdapter struct {
	tool   mcp.Tool
	client *mcpclient.Client
}

func NewMCToolAdapter(tool mcp.Tool, client *mcpclient.Client) *MCToolAdapter {
	return &MCToolAdapter{tool: tool, client: client}
}

// Name возвращает имя инструмента
func (a *MCToolAdapter) Name() string {
	return a.tool.Name
}

// Description возвращает описание инструмента
func (a *MCToolAdapter) Description() string {
	return a.tool.Description
}

// Call отправляет запрос к MCP и выполняет инструмент
func (a *MCToolAdapter) Call(ctx context.Context, input string) (string, error) {
	fmt.Printf("Calling tool %s with input: %s\n", a.tool.Name, input)
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to unmarshal input: %v", err)
	}

	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: a.tool.Name,
		},
		Params: mcp.CallToolParams{
			Name:      a.tool.Name,
			Arguments: params,
		},
	}

	resp, err := a.client.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to call tool %s: %w", a.tool.Name, err)
	}

	if resp.IsError {
		return "", fmt.Errorf("tool returned an error: %v", resp.Content)
	}

	var output string
	for _, content := range resp.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			output += textContent.Text + "\n"
		}
	}

	return output, nil
}
