module github.com/shadetheartist/ghost

go 1.19

require internal/ghost v1.0.0

replace internal/ghost => ./internal/ghost

require (
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	golang.org/x/sync v0.1.0
)
