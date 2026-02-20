package version

// Version is set at build time via -ldflags.
var Version string

func GetVersion() string {
	if Version == "" {
		return "development"
	}
	return Version
}
