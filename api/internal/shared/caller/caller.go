// Package caller identifies who performs a use case, so every write can be attributed.
package caller

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
