package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
)

func TestVMPowerCallsTheSubresourceOfTheMachine(t *testing.T) {
	tests := []struct {
		command string
		done    string
	}{
		{command: "start", done: "started"},
		{command: "stop", done: "stopped"},
		{command: "restart", done: "restarted"},
	}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			env := loggedIn(t)

			out, err := env.run(t, "vm", test.command, "clidbg-vm")
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}

			want := machinePath + "/clidbg-vm/" + test.command
			if !env.sent(http.MethodPost, want) {
				t.Errorf("no POST reached %q, the last request went to %q", want, env.lastPath())
			}
			if !strings.Contains(out, "virtualmachines/clidbg-vm "+test.done) {
				t.Errorf("vm %s = %q, want it to report the machine", test.command, out)
			}
		})
	}
}

func TestVMPowerReportsWhatTheServerAnswered(t *testing.T) {
	env := loggedIn(t)
	env.failures[machinePath+"/clidbg-vm/start"] = api.Response[any]{
		Message: `virtual machine "clidbg-vm" is already running`,
	}

	_, err := env.run(t, "vm", "start", "clidbg-vm")
	if err == nil {
		t.Fatal("run() error = nil, want the refusal of the server reported")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("run() error = %q, want the message of the server", err)
	}
}

func TestVMPowerNeedsAName(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "vm", "start"); err == nil {
		t.Fatal("run() error = nil, want a name asked for")
	}
}

func TestVMSerialNeedsATerminal(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "vm", "serial", "clidbg-vm")
	if err == nil {
		t.Fatal("run() error = nil, want a console without a terminal refused")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("run() error = %q, want it to name the terminal it has not", err)
	}
	for _, request := range env.requests {
		if strings.HasSuffix(request.URL.Path, "/serial") {
			t.Error("a console was opened although the input is no terminal")
		}
	}
}

func TestVMVNCRefusesAPortThatIsNone(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "vm", "vnc", "clidbg-vm", "--port", "70000"); err == nil {
		t.Fatal("run() error = nil, want a port outside the range refused")
	}
}
