package httpapi_test

import (
	"os"
	"strings"
	"testing"
)

// Every operation names the scope group an Access Token needs for it, and every read is a Read.
func TestEveryOperationHasAScopeGroup(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile("../../../../openapi/v1/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	inPaths, method := false, ""
	lines := strings.Split(string(spec), "\n")
	for i, l := range lines {
		switch {
		case l == "paths:":
			inPaths = true
		case inPaths && l != "" && l[0] != ' ':
			inPaths = false
		case inPaths && strings.HasPrefix(l, "    ") && !strings.HasPrefix(l, "     ") && strings.HasSuffix(l, ":"):
			method = strings.TrimSuffix(strings.TrimSpace(l), ":")
		case inPaths && strings.HasPrefix(l, "      operationId: "):
			group := strings.TrimPrefix(lines[i+1], "      x-ogen-operation-group: ")
			if group == lines[i+1] || !map[string]bool{"Read": true, "Build": true, "Play": true, "Account": true}[group] {
				t.Errorf("%s has no scope group", l)
			}
			if method == "get" && group != "Read" {
				t.Errorf("%s is a read in group %s", l, group)
			}
		}
	}
}
