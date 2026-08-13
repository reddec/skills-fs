// Package internal holds the generate directive for the OpenAPI server. The ogen-generated
// code lands in ./api; the hand-written implementation lives in ./server.
package internal

//go:generate go tool ogen --target ./api -package api --clean --config ../.ogen.yaml ../openapi.yaml
