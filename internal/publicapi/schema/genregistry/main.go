package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/publicapi/schema"
)

func main() {
	codes := schema.Codes()
	var b strings.Builder
	b.WriteString("[\n")
	for i, code := range codes {
		comma := ","
		if i == len(codes)-1 {
			comma = ""
		}
		if _, err := fmt.Fprintf(&b, "  { \"code\": %q }%s\n", code, comma); err != nil {
			os.Exit(1)
		}
	}
	b.WriteString("]\n")
	if err := os.MkdirAll("generated", 0o755); err != nil {
		os.Exit(1)
	}
	if err := os.WriteFile("generated/api-error-registry.json", []byte(b.String()), 0o644); err != nil {
		os.Exit(1)
	}
}
