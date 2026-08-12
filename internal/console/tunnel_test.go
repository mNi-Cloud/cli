package console

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func newTunnel(t *testing.T) *Tunnel {
	t.Helper()

	tunnel, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = tunnel.Close() })
	return tunnel
}

func TestTunnelJoinsAClientToTheConsole(t *testing.T) {
	tunnel := newTunnel(t)
	console := newFakeConsole("RFB 003.008\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() {
		served <- tunnel.Serve(ctx, func(context.Context) (io.ReadWriteCloser, error) { return console, nil })
	}()

	client, err := net.Dial("tcp", tunnel.Address())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	greeting := make([]byte, len("RFB 003.008\n"))
	if err := client.SetReadDeadline(time.Now().Add(waitTimeout)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	if _, err := io.ReadFull(client, greeting); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if string(greeting) != "RFB 003.008\n" {
		t.Errorf("the client read %q, want what the console said", greeting)
	}

	if _, err := client.Write([]byte("RFB 003.008\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	waitFor(t, func() bool { return console.sent() == "RFB 003.008\n" }, "what the client sent")

	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Errorf("Serve() error = %v, want a stopped tunnel to be no failure", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Serve() did not return after the context was done")
	}
}

func TestTunnelServesOneClientAfterAnother(t *testing.T) {
	tunnel := newTunnel(t)

	opened := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = tunnel.Serve(ctx, func(context.Context) (io.ReadWriteCloser, error) {
			opened <- struct{}{}
			return newFakeConsole(), nil
		})
	}()

	for range 2 {
		client, err := net.Dial("tcp", tunnel.Address())
		if err != nil {
			t.Fatalf("Dial() error = %v", err)
		}
		select {
		case <-opened:
		case <-time.After(waitTimeout):
			t.Fatal("no console was opened for the client")
		}
		_ = client.Close()
	}
}

func TestTunnelReportsAConsoleItCannotOpen(t *testing.T) {
	tunnel := newTunnel(t)
	refused := errors.New(`virtual machine "a" is not running`)

	served := make(chan error, 1)
	go func() {
		served <- tunnel.Serve(context.Background(), func(context.Context) (io.ReadWriteCloser, error) {
			return nil, refused
		})
	}()

	client, err := net.Dial("tcp", tunnel.Address())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case err := <-served:
		if !errors.Is(err, refused) {
			t.Errorf("Serve() error = %v, want the reason the console stayed shut", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Serve() did not report the console it could not open")
	}
}

func TestTunnelStopsBeforeItServesAnybody(t *testing.T) {
	tunnel := newTunnel(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := tunnel.Serve(ctx, func(context.Context) (io.ReadWriteCloser, error) {
		t.Error("a console was opened although the tunnel was already stopped")
		return newFakeConsole(), nil
	}); err != nil {
		t.Errorf("Serve() error = %v, want a stopped tunnel to be no failure", err)
	}
}

func TestListenRefusesAnAddressItCannotHave(t *testing.T) {
	if _, err := Listen("256.256.256.256:0"); err == nil {
		t.Fatal("Listen() error = nil, want an address that cannot be listened on refused")
	}
}
