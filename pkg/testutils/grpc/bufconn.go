package grpc

import (
	context "context"
	"net"

	"google.golang.org/grpc/test/bufconn"
)

// BuffconnDialer is a context dialer that can be used with the grpc.WithContextDialer DialOption
// It returns a connection to a buffconn Listener
func BuffconnDialer(l *bufconn.Listener) ContextDialer {
	return func(ctx context.Context, address string) (net.Conn, error) {
		return l.Dial()
	}
}
