// Command mcptools exports the MCP tools the OpenAPI spec declares with x-mcp.
package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mcpapi"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: mcptools <openapi.yaml> <tools.json>")
	}
	spec, err := os.ReadFile(os.Args[1]) //nolint:gosec // paths come from go:generate
	if err != nil {
		log.Fatal(err)
	}
	tools, err := mcpapi.Export(spec)
	if err != nil {
		log.Fatal(err)
	}
	out, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(os.Args[2], append(out, '\n'), 0o600); err != nil { //nolint:gosec // paths come from go:generate
		log.Fatal(err)
	}
}
