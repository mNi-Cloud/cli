package console

import (
	"context"
	"io"
	"net"
)

// Opener opens one connection to a console.
type Opener func(ctx context.Context) (io.ReadWriteCloser, error)

// Tunnel offers a console on a local port, for the clients that speak a
// protocol a terminal cannot show, such as the RFB protocol of a VNC console.
type Tunnel struct {
	listener net.Listener
}

// Listen takes the local address the console is offered on. An address whose
// port is zero is given a free one.
func Listen(address string) (*Tunnel, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	return &Tunnel{listener: listener}, nil
}

// Address is where a client connects to reach the console.
func (t *Tunnel) Address() string {
	return t.listener.Addr().String()
}

// Close gives up the local address.
func (t *Tunnel) Close() error {
	return t.listener.Close()
}

// Serve joins one client at a time to a console of its own, until ctx is done.
// The server hands out one console per machine, so a second client at the same
// time would take the console away from the first.
func (t *Tunnel) Serve(ctx context.Context, open Opener) error {
	defer stopOn(ctx, t.listener)()

	for {
		local, err := t.listener.Accept()
		if err != nil {
			return stopped(ctx, err)
		}

		err = t.serveOne(ctx, local, open)
		_ = local.Close()
		if err != nil {
			return stopped(ctx, err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (t *Tunnel) serveOne(ctx context.Context, local net.Conn, open Opener) error {
	remote, err := open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = remote.Close() }()

	defer stopOn(ctx, local, remote)()
	return join(local, remote)
}

// stopOn closes what is given once ctx is done, and hands back what stops
// watching. A read of a connection does not end on its own, so the connection
// is taken away from it.
func stopOn(ctx context.Context, closers ...io.Closer) func() {
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			for _, closer := range closers {
				_ = closer.Close()
			}
		case <-done:
		}
	}()

	return func() { close(done) }
}

// stopped reads a failure that a stopped tunnel caused as the end of the
// tunnel, and every other one as itself.
func stopped(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}
