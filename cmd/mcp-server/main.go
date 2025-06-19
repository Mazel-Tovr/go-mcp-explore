package main

import (
	"context"
	"fmt"
	"go-mcp/internal/tool"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create a new MCP mcp-server
	s := server.NewMCPServer(
		"Demo 🚀",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Add hello_world tool with enhanced description
	helloWorldTool := mcp.NewTool("hello_world",
		mcp.WithDescription("Use this tool to greet a person by name. Always use this when user asks to say hello or greet someone."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The name of the person to greet. Example: 'John', 'Alice', 'Анна'"),
		),
	)

	// Add calculator tool with enhanced description and parameter details
	//	calculatorTool := mcp.NewTool("calculate",
	//		mcp.WithDescription(`Use this tool for any mathematical calculations.
	//Available operations: addition (+), subtraction (-), multiplication (*), division (/).
	//Always use this tool when you need to perform any arithmetic operations or solve math problems.`),
	//		mcp.WithString("operation",
	//			mcp.Required(),
	//			mcp.Description(`The arithmetic operation to perform.
	//Must be one of: 'add' (for +), 'subtract' (for -), 'multiply' (for *), 'divide' (for /)`),
	//			mcp.Enum("add", "subtract", "multiply", "divide"),
	//		),
	//		mcp.WithNumber("x",
	//			mcp.Required(),
	//			mcp.Description("The first number in the operation. Can be integer or decimal."),
	//		),
	//		mcp.WithNumber("y",
	//			mcp.Required(),
	//			mcp.Description("The second number in the operation. Can be integer or decimal."),
	//		),
	//	)

	// Add tool handlers
	s.AddTool(helloWorldTool, helloHandler)
	//s.AddTool(calculatorTool, calculate)
	httpServer := server.NewStreamableHTTPServer(s)

	discoveryService := tool.NewDiscoveryService("http://localhost:9090/endpoints", 10*time.Second)
	_ = tool.NewMonitor(s, discoveryService)
	go discoveryService.Start(ctx)
	//monitor.Start(ctx)
	// Start the mcp-server
	fmt.Println("🚀 Server started on port 8080")
	if err := httpServer.Start(":8080"); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func calculate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fmt.Printf("calculate invoked with params: %v\n", request)

	op, err := request.RequireString("operation")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	x, err := request.RequireFloat("x")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	y, err := request.RequireFloat("y")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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
			return mcp.NewToolResultError("cannot divide by zero"), nil
		}
		result = x / y
	default:
		return mcp.NewToolResultError("invalid operation"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("%.2f", result)), nil
}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fmt.Printf("helloHandler invoked with params: %v\n", request)

	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello dear, %s! Glade to see you here!", name)), nil
}
