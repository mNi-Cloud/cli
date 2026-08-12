package buildinfo

import "testing"

const revision = "914af9f817fdc3321b301f6a6faf192af74f5071"

func TestVersionTakesWhatTheLinkerStamped(t *testing.T) {
	got := version("v0.2.0-3-g914af9f", VCS{Revision: revision, Time: "2026-08-08T10:48:25Z"})

	if want := "v0.2.0-3-g914af9f"; got != want {
		t.Errorf("version() = %q, want %q", got, want)
	}
}

func TestVersionOfACheckout(t *testing.T) {
	got := version("", VCS{Revision: revision, Time: "2026-08-08T10:48:25Z"})

	if want := "914af9f817fd (2026-08-08T10:48:25Z)"; got != want {
		t.Errorf("version() = %q, want %q", got, want)
	}
}

func TestVersionOfACheckoutWithChangesInIt(t *testing.T) {
	got := version("", VCS{Revision: revision, Modified: true, Time: "2026-08-08T10:48:25Z"})

	if want := "914af9f817fd-dirty (2026-08-08T10:48:25Z)"; got != want {
		t.Errorf("version() = %q, want %q", got, want)
	}
}

func TestVersionWithoutTheTimeOfTheCommit(t *testing.T) {
	got := version("", VCS{Revision: revision})

	if want := "914af9f817fd"; got != want {
		t.Errorf("version() = %q, want %q", got, want)
	}
}

// A build made with -buildvcs=false knows nothing about where it comes from,
// and says so rather than making a version up.
func TestVersionOfABuildThatKnowsNothing(t *testing.T) {
	got := version("", VCS{})

	if got != Unknown {
		t.Errorf("version() = %q, want %q", got, Unknown)
	}
}

// urfave/cli only offers --version once the version is not empty, so an empty
// answer would take the flag away from the binary.
func TestVersionIsNeverEmpty(t *testing.T) {
	if Version("") == "" {
		t.Error("Version() = \"\", want something the binary can report")
	}
}
