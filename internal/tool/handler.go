package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	mcptool "github.com/ckanthony/openapi-mcp/pkg/mcp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// genericHandler is a reusable function that creates a handler for a specific tool.
func NewGenericToolHandler(toolSet *mcptool.ToolSet, toolName string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Step 1: Retrieve the operation details for the tool
		op, exists := toolSet.Operations[toolName]
		if !exists {
			return nil, fmt.Errorf("operation not found for tool %s", toolName)
		}

		// Step 2: Extract arguments from the request
		args := request.GetArguments()
		if args == nil && len(op.Parameters) > 0 {
			return nil, fmt.Errorf("no arguments provided for tool %s", toolName)
		}

		// Step 3: Build the full URL with path parameters
		fullURL := strings.Replace(op.BaseURL, "https", "http", 1) + op.Path

		// Step 4: Apply path parameters
		for _, param := range op.Parameters {
			if param.In == "path" {
				paramValue, ok := args[param.Name]
				if !ok {
					return nil, fmt.Errorf("missing path parameter %s for tool %s", param.Name, toolName)
				}
				fullURL = strings.Replace(fullURL, "{"+param.Name+"}", fmt.Sprintf("%v", paramValue), 1)
			}
		}

		// Step 5: Prepare query parameters
		query := url.Values{}
		for _, param := range op.Parameters {
			if param.In == "query" {
				paramValue, ok := args[param.Name]
				if !ok {
					continue
				}
				query.Add(param.Name, fmt.Sprintf("%v", paramValue))
			}
		}

		// Step 6: Prepare headers
		req, err := http.NewRequestWithContext(ctx, op.Method, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		for _, param := range op.Parameters {
			if param.In == "header" {
				for key, value := range args {
					req.Header.Set(key, fmt.Sprintf("%v", value))
				}
			}
		}

		// Step 7: Handle request body parameters
		bodyParams := make(map[string]interface{})
		for _, param := range op.Parameters {
			if param.In == "body" {
				for key, value := range args {
					bodyParams[key] = value
				}
			}
		}

		if len(bodyParams) > 0 {
			bodyBytes, err := json.Marshal(bodyParams)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
		}

		// Step 8: Handle API key if present
		//apiKeyName, apiKeyIn := toolSet.GetAPIKeyDetails()
		//if apiKeyName != "" {
		//	apiKeyValue, ok := args[apiKeyName]
		//	if !ok {
		//		return nil, fmt.Errorf("missing API key %s", apiKeyName)
		//	}
		//	if apiKeyIn == "header" {
		//		req.Header.Set(apiKeyName, fmt.Sprintf("%v", apiKeyValue))
		//	} else if apiKeyIn == "query" {
		//		query.Add(apiKeyName, fmt.Sprintf("%v", apiKeyValue))
		//	}
		//}

		req.URL.RawQuery = query.Encode()

		// Step 9: Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		// Step 10: Read and parse the response
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return mcp.NewToolResultError(fmt.Sprintf("request failed with status code %d", resp.StatusCode)), nil
		}

		//var result string
		//if err = json.Unmarshal(respBody, &result); err != nil {
		//	return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
		//}

		return mcp.NewToolResultText(string(respBody)), nil
	}
}
