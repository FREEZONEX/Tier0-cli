package version

// BuildVersion is injected through go build -ldflags.
// go build -ldflags "-X github.com/FREEZONEX/Tier0-cli/internal/version.BuildVersion=v0.1.0"
var BuildVersion = "dev"

// BuildCommit is injected through go build -ldflags.
var BuildCommit = "unknown"

// BuildDate is injected through go build -ldflags.
var BuildDate = "unknown"

// IsDev reports whether this is a development build.
func IsDev() bool {
	return BuildVersion == "" || BuildVersion == "dev"
}
