package version

// BuildVersion 通过 go build -ldflags 注入
// go build -ldflags "-X github.com/FREEZONEX/Tier0-cli/internal/version.BuildVersion=v0.1.0"
var BuildVersion = "dev"

// BuildCommit 通过 go build -ldflags 注入
var BuildCommit = "unknown"

// BuildDate 通过 go build -ldflags 注入
var BuildDate = "unknown"

// IsDev 判断是否为开发版本
func IsDev() bool {
	return BuildVersion == "" || BuildVersion == "dev"
}
