package server

import "context"

// ServerRunner listens on a Unix socket and serves JSON-RPC requests.
// Serve blocks until ctx is canceled or the socket is closed.
// Example: err := runner.Serve(context.Background()).
type ServerRunner interface {
	Serve(ctx context.Context) error
}
