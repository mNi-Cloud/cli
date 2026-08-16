package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/console"
	"github.com/urfave/cli/v3"
)

const (
	logsPath = containerPath + "/clidbg-ctr/logs"
	execPath = containerPath + "/clidbg-ctr/exec"
)

func execFrame(t *testing.T, channel byte, payload string) []byte {
	t.Helper()
	return append([]byte{channel}, payload...)
}

func execStatus(t *testing.T, status console.ExitStatus) []byte {
	t.Helper()

	payload, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return append([]byte{console.ChannelStatus}, payload...)
}

func TestContainerLogsWritesWhatTheServerStreams(t *testing.T) {
	env := loggedIn(t)
	env.streams[logsPath] = [][]byte{[]byte("first line\n"), []byte("second line\n")}

	out, errOut, err := env.runSplit(t, "ctr", "logs", "clidbg-ctr")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if out != "first line\nsecond line\n" {
		t.Errorf("logs = %q, want the lines the server streamed", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want the logs written to the output alone", errOut)
	}
}

func TestContainerLogsWithoutFlagsAsksForNothing(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "ctr", "logs", "clidbg-ctr"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if got := env.queryOf(logsPath); len(got) != 0 {
		t.Errorf("query = %v, want none", got)
	}
}

func TestContainerLogsCarriesEveryFlagInTheQuery(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "ctr", "logs", "clidbg-ctr",
		"--follow", "--tail", "20", "--timestamps", "--previous", "--since", "5m")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	query := env.queryOf(logsPath)
	want := map[string]string{
		"follow":     "true",
		"tail":       "20",
		"timestamps": "true",
		"previous":   "true",
		"since":      "5m",
	}
	for name, value := range want {
		if got := query.Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestContainerLogsTakesTheShortFollowFlag(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "ctr", "logs", "clidbg-ctr", "-f"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if got := env.queryOf(logsPath).Get("follow"); got != "true" {
		t.Errorf("follow = %q, want %q", got, "true")
	}
}

func TestContainerLogsRefusesASinceThatIsNoLengthOfTime(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "ctr", "logs", "clidbg-ctr", "--since", "yesterday")
	if err == nil {
		t.Fatal("run() error = nil, want a length of time that is none refused")
	}
	if !strings.Contains(err.Error(), "yesterday") {
		t.Errorf("run() error = %q, want it to name what was typed", err)
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the flag is no length of time", len(env.requests))
	}
}

func TestContainerLogsRefusesANegativeTail(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "ctr", "logs", "clidbg-ctr", "--tail", "-1"); err == nil {
		t.Fatal("run() error = nil, want a number of lines that is none refused")
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the flag is no number of lines", len(env.requests))
	}
}

func TestContainerLogsNeedsAName(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "ctr", "logs"); err == nil {
		t.Fatal("run() error = nil, want a name asked for")
	}
}

func TestContainerLogsReportsWhatTheServerAnswered(t *testing.T) {
	env := loggedIn(t)
	env.failures[logsPath] = api.Response[any]{Message: `container "clidbg-ctr" is not running`}

	_, err := env.run(t, "ctr", "logs", "clidbg-ctr")
	if err == nil {
		t.Fatal("run() error = nil, want the refusal of the server reported")
	}
	if !strings.Contains(UserFacing(err).Error(), "is not running") {
		t.Errorf("run() error = %q, want the message of the server", err)
	}
}

func TestContainerExecCarriesTheCommandInTheQuery(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{execStatus(t, console.ExitStatus{})}

	if _, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "sh", "-c", "echo hello"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	query := env.queryOf(execPath)
	command := query["command"]
	if len(command) != 3 || command[0] != "sh" || command[1] != "-c" || command[2] != "echo hello" {
		t.Errorf("command = %v, want the words in the order they were typed", command)
	}
	if got := query.Get("stdin"); got != "" {
		t.Errorf("stdin = %q, want nothing without --stdin", got)
	}
	if got := query.Get("tty"); got != "" {
		t.Errorf("tty = %q, want nothing without --tty", got)
	}
}

func TestContainerExecWritesEachOutputWhereItBelongs(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{
		execFrame(t, console.ChannelStdout, "what it wrote"),
		execFrame(t, console.ChannelStderr, "what went wrong"),
		execStatus(t, console.ExitStatus{}),
	}

	out, errOut, err := env.runSplit(t, "ctr", "exec", "clidbg-ctr", "--", "sh")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if out != "what it wrote" {
		t.Errorf("stdout = %q, want what the command wrote", out)
	}
	if errOut != "what went wrong" {
		t.Errorf("stderr = %q, want what went wrong", errOut)
	}
}

func TestContainerExecEndsWithTheExitCodeOfTheCommand(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{execStatus(t, console.ExitStatus{ExitCode: 3})}

	_, errOut, err := env.runSplit(t, "ctr", "exec", "clidbg-ctr", "--", "false")
	if err == nil {
		t.Fatal("run() error = nil, want the exit code of the command carried out")
	}

	var exitCoder cli.ExitCoder
	if !errors.As(err, &exitCoder) {
		t.Fatalf("run() error = %v, want an error that carries an exit code", err)
	}
	if exitCoder.ExitCode() != 3 {
		t.Errorf("ExitCode() = %d, want 3", exitCoder.ExitCode())
	}
	if ExitCode(err) != 3 {
		t.Errorf("ExitCode() = %d, want 3", ExitCode(err))
	}
	if UserFacing(err) != nil {
		t.Errorf("UserFacing() = %v, want nothing written about a command that said it all", UserFacing(err))
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing beside what the command wrote", errOut)
	}
}

func TestContainerExecKeepsTheHighestExitCodeACommandCanHave(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{execStatus(t, console.ExitStatus{ExitCode: 255})}

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "false")
	if err == nil {
		t.Fatal("run() error = nil, want the exit code of the command carried out")
	}
	if ExitCode(err) != 255 {
		t.Errorf("ExitCode() = %d, want 255 told apart from a session that broke", ExitCode(err))
	}
}

// TestContainerExecDoesNotPassOnAnExitCodeThatIsNone keeps a session that broke
// apart from a command that ended with 255, which is what an exit code below
// zero would look like to the shell.
func TestContainerExecDoesNotPassOnAnExitCodeThatIsNone(t *testing.T) {
	tests := []struct {
		name   string
		status console.ExitStatus
		says   string
	}{
		{
			name:   "with a message",
			status: console.ExitStatus{ExitCode: -1, Message: "the session ended before the command did"},
			says:   "the session ended before the command did",
		},
		{
			name:   "without a message",
			status: console.ExitStatus{ExitCode: -1},
			says:   "did not say how the command ended",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := loggedIn(t)
			env.streams[execPath] = [][]byte{execStatus(t, test.status)}

			_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "sh")
			if err == nil {
				t.Fatal("run() error = nil, want the broken session reported")
			}
			if ExitCode(err) != 1 {
				t.Errorf("ExitCode() = %d, want 1, because an exit code below zero is none of a command", ExitCode(err))
			}

			message := UserFacing(err)
			if message == nil || !strings.Contains(message.Error(), test.says) {
				t.Errorf("UserFacing() = %v, want it to say %q", message, test.says)
			}
		})
	}
}

func TestContainerExecEndsWithoutAnErrorWhenTheCommandWorked(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{execStatus(t, console.ExitStatus{})}

	if _, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "true"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if ExitCode(nil) != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", ExitCode(nil))
	}
}

func TestContainerExecReportsWhyACommandNeverRan(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{
		execStatus(t, console.ExitStatus{ExitCode: 126, Message: "cannot run the command"}),
	}

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "nope")
	if err == nil {
		t.Fatal("run() error = nil, want the failure of the command reported")
	}
	if !strings.Contains(err.Error(), "cannot run the command") {
		t.Errorf("run() error = %q, want the message of the server", err)
	}
	if ExitCode(err) != 126 {
		t.Errorf("ExitCode() = %d, want 126", ExitCode(err))
	}
}

func TestContainerExecNeedsACommand(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr")
	if err == nil {
		t.Fatal("run() error = nil, want a command asked for")
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although there is no command to run", len(env.requests))
	}
}

func TestContainerExecNeedsAName(t *testing.T) {
	env := loggedIn(t)

	if _, err := env.run(t, "ctr", "exec"); err == nil {
		t.Fatal("run() error = nil, want a name asked for")
	}
}

func TestContainerExecRefusesATerminalWithoutInput(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--tty", "--", "sh")
	if err == nil {
		t.Fatal("run() error = nil, want a terminal without input refused")
	}
	if !strings.Contains(err.Error(), "--stdin") {
		t.Errorf("run() error = %q, want it to name the flag that is missing", err)
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the flags cannot go together", len(env.requests))
	}
}

func TestContainerExecNeedsATerminalToGiveTheCommandOne(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--stdin", "--tty", "--", "sh")
	if err == nil {
		t.Fatal("run() error = nil, want a terminal that cannot be given refused")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("run() error = %q, want it to name the terminal it has not", err)
	}
	if len(env.requests) != 0 {
		t.Errorf("%d requests were sent although the input is no terminal", len(env.requests))
	}
}

// TestContainerExecTakesTheShortFlagsWrittenTogether keeps `-it` reading as
// `--stdin --tty`, the line every user of kubectl writes. It is refused here
// only because the test has no terminal, which is how the long form ends too.
func TestContainerExecTakesTheShortFlagsWrittenTogether(t *testing.T) {
	env := loggedIn(t)

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "-it", "--", "sh")
	if err == nil {
		t.Fatal("run() error = nil, want a terminal that cannot be given refused")
	}
	if !strings.Contains(err.Error(), "not a terminal") {
		t.Errorf("run() error = %q, want -it to read as --stdin --tty", err)
	}
}

// TestContainerExecTakesInputThatIsNoTerminal keeps a pipe working, which is
// what --stdin on its own is for: `mni ctr exec -i web -- cat < file`.
func TestContainerExecTakesInputThatIsNoTerminal(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{
		execFrame(t, console.ChannelStdout, "hello"),
		execStatus(t, console.ExitStatus{}),
	}

	out, _, err := env.runSplitWith(t, strings.NewReader("hello"), "ctr", "exec", "clidbg-ctr", "--stdin", "--", "cat")
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if got := env.queryOf(execPath).Get("stdin"); got != "true" {
		t.Errorf("stdin = %q, want %q", got, "true")
	}
	if got := env.queryOf(execPath).Get("tty"); got != "" {
		t.Errorf("tty = %q, want nothing without --tty", got)
	}
	if out != "hello" {
		t.Errorf("stdout = %q, want what the command wrote", out)
	}
}

// TestContainerExecSaysWhenPipedInputEnded holds the answer of the server back
// until the client says that the input ended, the way `cat` waits for the end
// of what is piped into it. A client that never says it leaves such a run
// hanging.
func TestContainerExecSaysWhenPipedInputEnded(t *testing.T) {
	env := loggedIn(t)
	env.awaitFrame[execPath] = []byte{console.ChannelStdinClose}
	env.streams[execPath] = [][]byte{
		execFrame(t, console.ChannelStdout, "hello"),
		execStatus(t, console.ExitStatus{ExitCode: 4}),
	}

	out, _, err := env.runSplitWith(t, strings.NewReader("hello"), "ctr", "exec", "clidbg-ctr", "--stdin", "--", "cat")
	if ExitCode(err) != 4 {
		t.Fatalf("ExitCode() = %d, want the code the command ended with after its input ended", ExitCode(err))
	}
	if out != "hello" {
		t.Errorf("stdout = %q, want what the command wrote", out)
	}

	frames := env.framesOf(execPath)
	if len(frames) == 0 {
		t.Fatal("the server was sent nothing, want the input and the end of it")
	}
	if got, want := frames[0], execFrame(t, console.ChannelStdin, "hello"); !bytes.Equal(got, want) {
		t.Errorf("the server was sent %q first, want %q", got, want)
	}

	ends := 0
	for _, sent := range frames {
		if bytes.Equal(sent, []byte{console.ChannelStdinClose}) {
			ends++
		}
	}
	if ends != 1 {
		t.Errorf("the end of the input was sent %d times, want once", ends)
	}
}

// TestContainerExecTakesTheTenantByItsLongNameOnly keeps the reading of -t a
// decision somebody makes: on this command it is --tty, the way kubectl reads
// it, so the tenant needs the name it is written out with.
func TestContainerExecTakesTheTenantByItsLongNameOnly(t *testing.T) {
	env := loggedIn(t)
	otherPath := "/ctr/v1alpha/tenants/other/containers/clidbg-ctr/exec"
	env.streams[otherPath] = [][]byte{execStatus(t, console.ExitStatus{})}

	if _, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--tenant", "other", "--", "sh"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	if !env.sent(http.MethodGet, otherPath) {
		t.Errorf("no stream was opened on %q, the last request went to %q", otherPath, env.lastPath())
	}
}

func TestContainerExecReportsWhatTheServerAnswered(t *testing.T) {
	env := loggedIn(t)
	env.failures[execPath] = api.Response[any]{Message: `container "clidbg-ctr" is not running`}

	_, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "sh")
	if err == nil {
		t.Fatal("run() error = nil, want the refusal of the server reported")
	}
	if !strings.Contains(UserFacing(err).Error(), "is not running") {
		t.Errorf("run() error = %q, want the message of the server", err)
	}
}

func TestContainerLogsAndExecAskForAStream(t *testing.T) {
	env := loggedIn(t)
	env.streams[execPath] = [][]byte{execStatus(t, console.ExitStatus{})}

	if _, err := env.run(t, "ctr", "logs", "clidbg-ctr"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if _, err := env.run(t, "ctr", "exec", "clidbg-ctr", "--", "sh"); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	for _, path := range []string{logsPath, execPath} {
		if !env.sent(http.MethodGet, path) {
			t.Errorf("no stream was opened on %q, the last request went to %q", path, env.lastPath())
		}
	}
}
