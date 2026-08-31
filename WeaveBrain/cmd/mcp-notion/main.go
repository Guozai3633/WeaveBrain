package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"notion",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("create_notion_page", mcp.WithDescription("Create a new page in a Notion database"),
			mcp.WithString("database_id", mcp.Required(), mcp.Description("The ID of the Notion database to create the page in")),
			mcp.WithString("title", mcp.Required(), mcp.Description("The title of the new page")),
			mcp.WithString("content", mcp.Description("Optional content for the page as a paragraph")),
			mcp.WithString("_credentials", mcp.Description("Auto-injected Notion API token")),
		),
		createNotionPageHandler,
	)

	server.ServeStdio(s)
}

func createNotionPageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}

	dbID, ok := args["database_id"].(string)
	if !ok {
		return mcp.NewToolResultError("database_id is required"), nil
	}

	title, ok := args["title"].(string)
	if !ok {
		return mcp.NewToolResultError("title is required"), nil
	}

	content, _ := args["content"].(string)
	credentials, ok := args["_credentials"].(string)
	if !ok || credentials == "" {
		return mcp.NewToolResultError("Notion API credentials missing. Please configure Notion integration first."), nil
	}

	// Build Notion API request body
	payload := map[string]interface{}{
		"parent": map[string]string{"database_id": dbID},
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"title": []map[string]interface{}{
					{
						"text": map[string]string{"content": title},
					},
				},
			},
		},
	}

	if content != "" {
		payload["children"] = []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{
							"type": "text",
							"text": map[string]string{"content": content},
						},
					},
				},
			},
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal request payload: %v", err)), nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create request: %v", err)), nil
	}

	req.Header.Set("Authorization", "Bearer "+credentials)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send request to Notion API: %v", err)), nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return mcp.NewToolResultError(fmt.Sprintf("Notion API returned error status %d: %s", resp.StatusCode, string(bodyBytes))), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created Notion page. Response: %s", string(bodyBytes))), nil
}
