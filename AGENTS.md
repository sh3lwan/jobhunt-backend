# AGENTS.md - JobHunter Backend

## Commands
- Build: `go build -o ./tmp/app ./cmd/main.go` 
- Run: `make air` (live reload) or `go run cmd/main.go`
- Test: `go test ./...` (single test: `go test -run TestName ./path/to/package`)
- Database: `make migrate-up` / `make migrate-down`
- Generate SQL: `sqlc generate`

## Code Style
- **Imports**: Standard library first, then third-party, then local imports with blank lines between groups
- **Naming**: CamelCase for exported, camelCase for unexported, use descriptive names (e.g., `CVService`, `dbJobService`)
- **Structs**: JSON tags for all exported fields, comments for exported types
- **Error Handling**: Always check errors, use `fmt.Errorf` for wrapping, log errors before returning
- **HTTP Handlers**: Use `utils.RespondJSON()` for responses, validate inputs, handle context cancellation
- **Database**: Use sqlc-generated queries, always pass context, use transactions for multi-step operations
- **Services**: Dependency injection via constructors (e.g., `NewHandler()`), separate business logic from HTTP handling
- **Concurrency**: Use goroutines for async tasks (job fetching, embedding), proper context handling for graceful shutdown
- **Types**: Use appropriate types (`int32` for IDs, `time.Time` for timestamps), prefer structs over maps for structured data