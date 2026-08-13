// Package embeddedskill exposes the trusted Tier0 Skill baseline compiled into
// every CLI binary. The installed Skill may be updated independently; this
// package exists so a compatible baseline is always available for repair.
package embeddedskill

import (
	"embed"
	"io/fs"
)

// content is deliberately populated by scripts/sync-embedded-skill.sh using a
// runtime-only whitelist. Maintainer snapshots and repository metadata are not
// embedded.
//
//go:embed content content/_source.json
var content embed.FS

// FS returns the root of the embedded Tier0 Skill package.
func FS() (fs.FS, error) {
	return fs.Sub(content, "content")
}
