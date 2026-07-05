When you change Go code, before stopping, test what you changed, run go fmt, go fix, and golangci-lint.

When generating skills or any other files for temporary testing or exploration, use the `debug` directory. Manual acceptance check: `go run . generate https://github.com/longhorn/website --content-path kb -o debug/longhorn-kb`.

Never create git tags or releases — releasing is done manually by Yarden. Running `goreleaser check` as validation is fine.
