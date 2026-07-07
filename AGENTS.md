When you change Go code, before stopping, test what you changed, run go fmt, go fix, golangci-lint, and `go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...` to apply modern Go idioms.

When generating skills or any other files for temporary testing or exploration, use the `debug` directory.

