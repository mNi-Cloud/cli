// Package buildinfo works out what version a build of the CLI reports.
package buildinfo

import "runtime/debug"

// Unknown is what a build reports when neither the linker nor the version
// control system said where it comes from.
const Unknown = "unknown"

// revisionLength is how much of a commit hash the version carries. It is the
// length Go itself writes a revision in a module version with.
const revisionLength = 12

// VCS is what the toolchain recorded about the checkout a binary was built
// from. Go fills it in for a build made inside a repository.
type VCS struct {
	Revision string
	Modified bool
	Time     string
}

// Version returns the version of this binary. The argument is what -ldflags
// stamped in, and is empty in a build that stamped nothing.
func Version(stamped string) string {
	return version(stamped, readVCS())
}

func version(stamped string, vcs VCS) string {
	if stamped != "" {
		return stamped
	}
	if vcs.Revision == "" {
		return Unknown
	}

	built := shorten(vcs.Revision)
	if vcs.Modified {
		built += "-dirty"
	}
	if vcs.Time != "" {
		built += " (" + vcs.Time + ")"
	}
	return built
}

func readVCS() VCS {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return VCS{}
	}

	var vcs VCS
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			vcs.Revision = setting.Value
		case "vcs.modified":
			vcs.Modified = setting.Value == "true"
		case "vcs.time":
			vcs.Time = setting.Value
		}
	}
	return vcs
}

func shorten(revision string) string {
	if len(revision) <= revisionLength {
		return revision
	}
	return revision[:revisionLength]
}
