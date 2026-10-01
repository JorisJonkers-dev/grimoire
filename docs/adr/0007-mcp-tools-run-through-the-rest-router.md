# MCP tools run through the REST router

An MCP tool is an OpenAPI operation marked with `x-mcp`. `task gen` exports each one's input schema
(its path and query parameters plus its request body, with every `$ref` inlined) into
`mcpapi/tools.json`. When an agent calls a tool, the MCP endpoint builds the matching HTTP request
and serves it in-process through the same generated router as REST. The caller's identity and the
`mcp` origin travel in the request context.

A tool and its REST twin therefore cannot disagree on validation, authorisation, error messages or
Revisions, and adding a tool takes one line in the spec. An earlier plan had a hand-written adapter
per tool calling the use cases. That would have duplicated every converter and every error mapping.

The costs:

- Each call is encoded to JSON and decoded again inside the process.
- Tool names follow the operations (`create_npc`, `update_npc`) instead of one `upsert_npc`.
- A tool that is not a single operation would need its own endpoint first.
