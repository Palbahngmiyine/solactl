// Package version provides build-time version information.
package version

import "runtime/debug"

var (
	Version = ""
	Commit  = ""
	Date    = ""
)

func init() {
	info, ok := debug.ReadBuildInfo()

	if Version == "" {
		if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		} else {
			Version = "dev"
		}
	}

	if Commit == "" || Date == "" {
		var dirty bool
		if ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					if Commit == "" {
						Commit = s.Value
					}
				case "vcs.time":
					if Date == "" {
						Date = s.Value
					}
				case "vcs.modified":
					dirty = s.Value == "true"
				}
			}
		}
		if Commit == "" {
			Commit = "none"
		} else if dirty {
			Commit += "-dirty"
		}
		if Date == "" {
			Date = "unknown"
		}
	}
}
