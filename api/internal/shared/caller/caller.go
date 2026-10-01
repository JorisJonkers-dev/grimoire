// Package caller identifies who performs a use case, so every write can be attributed.
package caller

import "context"

// Origin is where a request came from.
type Origin string

// Origins a Caller can have.
const (
	OriginUI        Origin = "ui"
	OriginMCP       Origin = "mcp"
	OriginGenerator Origin = "generator"
	OriginSystem    Origin = "system"
)

// Caller is the authenticated account behind a use case and the surface it used.
type Caller struct {
	Subject string
	Origin  Origin
	// Client names the MCP client or generator, when there is one.
	Client string
}

// UI is a caller acting through the web app.
func UI(subject string) Caller {
	return Caller{Subject: subject, Origin: OriginUI}
}

// MCP is a caller acting through an MCP client.
func MCP(subject, client string) Caller {
	return Caller{Subject: subject, Origin: OriginMCP, Client: client}
}

type clientKey struct{}

// WithMCPClient marks a request as made through the named MCP client.
func WithMCPClient(ctx context.Context, client string) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

// MCPClient names the MCP client behind a request, if it came through one.
func MCPClient(ctx context.Context) (string, bool) {
	client, ok := ctx.Value(clientKey{}).(string)
	return client, ok
}
