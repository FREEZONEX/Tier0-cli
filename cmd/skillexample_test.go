// Run this test:
//
//	go test ./cmd/... -run TestSkillExampleCommands -v
//
// The test scans every *.md under the sibling Tier0-skill directory, extracts
// all "tier0 ..." invocations, and validates each command path and flag
// against the live cobra tree. Any unknown command or unrecognised flag is
// reported as a test failure.
package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestSkillExampleCommands validates every "tier0 ..." invocation found in
// Tier0-skill markdown files against the live cobra command tree.
//
// A mismatch — an unknown subcommand or an unrecognised flag — fails the test,
// preventing skill docs from drifting out of sync with the CLI.
func TestSkillExampleCommands(t *testing.T) {
	skillDir := findSkillDir(t)
	cat := buildSkillExampleCatalog()

	type issue struct {
		file, problem, raw string
		line               int
	}
	var issues []issue

	err := filepath.WalkDir(skillDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(skillDir, path)
		for _, ref := range parseSkillExampleRefs(string(data)) {
			cmdPath, ok := cat.resolve(ref.words)
			if !ok {
				issues = append(issues, issue{
					file:    rel,
					line:    ref.line,
					raw:     ref.raw,
					problem: "unknown command: tier0 " + strings.Join(ref.words, " "),
				})
				continue
			}
			for _, f := range ref.flags {
				if !cat.hasFlag(cmdPath, f) {
					issues = append(issues, issue{
						file:    rel,
						line:    ref.line,
						raw:     ref.raw,
						problem: "unknown flag " + f + " on \"tier0 " + cmdPath + "\"",
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range issues {
		t.Errorf("%s:%d  %s\n    %s", iss.file, iss.line, iss.problem, strings.TrimSpace(iss.raw))
	}
	if t.Failed() {
		t.Fatalf("skill doc examples are out of sync with the CLI — fix the docs or the command definitions")
	}
}

// ── catalog ───────────────────────────────────────────────────────────────────

type skillExampleCatalog struct {
	flags  map[string]map[string]bool // command path → accepted flag tokens
	groups map[string]bool            // command paths that have subcommands
}

// buildSkillExampleCatalog walks the live rootCmd tree and records every
// command path and its accepted flags (local + inherited).
func buildSkillExampleCatalog() *skillExampleCatalog {
	cat := &skillExampleCatalog{
		flags:  map[string]map[string]bool{},
		groups: map[string]bool{},
	}
	var walk func(c *cobra.Command, prefix string)
	walk = func(c *cobra.Command, prefix string) {
		use := c.Use
		if i := strings.IndexAny(use, " \t["); i > 0 {
			use = use[:i]
		}
		var path string
		if c == rootCmd {
			path = ""
		} else {
			path = strings.TrimSpace(prefix + " " + use)
		}

		flagSet := map[string]bool{}
		add := func(fl *pflag.Flag) {
			flagSet["--"+fl.Name] = true
			if fl.Shorthand != "" {
				flagSet["-"+fl.Shorthand] = true
			}
		}
		c.Flags().VisitAll(add)
		c.InheritedFlags().VisitAll(add)

		cat.flags[path] = flagSet
		if c.HasSubCommands() {
			cat.groups[path] = true
		}
		for _, sub := range c.Commands() {
			walk(sub, path)
		}
	}
	walk(rootCmd, "")
	return cat
}

// resolve finds the longest matching command path from words (e.g. ["uns","browse"]).
// Returns ("uns browse", true) if "uns browse" is in the catalog.
func (c *skillExampleCatalog) resolve(words []string) (string, bool) {
	for i := len(words); i > 0; i-- {
		path := strings.Join(words[:i], " ")
		if _, ok := c.flags[path]; ok {
			// If it's a group and there are more words, those are an unknown sub-command.
			if i < len(words) && c.groups[path] {
				return "", false
			}
			return path, true
		}
	}
	return "", false
}

func (c *skillExampleCatalog) hasFlag(cmdPath, flag string) bool {
	return c.flags[cmdPath][flag]
}

// ── markdown parser ───────────────────────────────────────────────────────────

type skillExampleRef struct {
	line  int
	raw   string
	words []string // subcommand words after "tier0" (before first flag)
	flags []string // flag tokens, e.g. "--path", "-p"
}

var (
	// tier0Token matches "tier0" as a standalone word.
	tier0TokenRe = regexp.MustCompile(`(?:^|[ \t` + "`" + `])tier0(?:[ \t]|$)`)
	// subCmdWord: a valid subcommand segment is lowercase letters/digits/hyphens.
	subCmdWordRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// parseSkillExampleRefs extracts every tier0 command invocation from markdown.
// Handles backslash line continuations. Skips shell comments (# …).
func parseSkillExampleRefs(content string) []skillExampleRef {
	var refs []skillExampleRef
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		logical := lines[i]
		lineNo := i + 1
		// Join backslash-continued lines.
		for strings.HasSuffix(strings.TrimRight(logical, " \t"), `\`) {
			trimmed := strings.TrimRight(logical, " \t")
			logical = trimmed[:len(trimmed)-1] + " "
			i++
			if i < len(lines) {
				logical += strings.TrimLeft(lines[i], " \t")
			}
		}
		if !tier0TokenRe.MatchString(logical) {
			continue
		}
		idx := strings.Index(logical, "tier0")
		after := logical[idx+5:]
		after = strings.TrimLeft(after, " \t")
		// Strip trailing inline comment.
		if ci := strings.Index(after, " #"); ci >= 0 {
			after = after[:ci]
		}
		// Strip trailing backtick/quote that may close an inline code span.
		after = strings.TrimRight(after, "`'\"")

		if ref, ok := parseSkillExampleCmd(after, lineNo, logical[idx:]); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

// relativeTimeRe matches flag-value-shaped tokens like -1h, -30m, -7d.
// These follow --start / --end and must not be treated as flags.
var relativeTimeRe = regexp.MustCompile(`^-\d`)

// extractFlagName returns just the flag name portion of a token.
// It accepts --long-name or -s and stops at the first character that is not
// a letter, digit, or hyphen — stripping markdown artifacts like --json`，...
func extractFlagName(tok string) string {
	if strings.HasPrefix(tok, "--") {
		end := 2
		for end < len(tok) && isFlagNameChar(tok[end]) {
			end++
		}
		return tok[:end]
	}
	// Short flag: -[a-zA-Z]
	if len(tok) >= 2 {
		return tok[:2]
	}
	return tok
}

func isFlagNameChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-'
}

func parseSkillExampleCmd(after string, lineNo int, raw string) (skillExampleRef, bool) {
	tokens := strings.Fields(after)
	if len(tokens) == 0 {
		return skillExampleRef{}, false
	}

	var words, flags []string
	passedSubcmds := false

	for _, tok := range tokens {
		// Clean markdown decoration: strip leading [ and trailing ]`'")
		tok = strings.TrimLeft(tok, "[")
		tok = strings.TrimRight(tok, "]`'\")")
		if tok == "" {
			continue
		}
		// Skip doc-notation tokens like "--source|--event" or bare "|".
		if strings.ContainsAny(tok, "|") {
			passedSubcmds = true
			continue
		}

		if strings.HasPrefix(tok, "-") {
			// Skip relative-time values that look like flags (-1h, -24h, -7d).
			if relativeTimeRe.MatchString(tok) {
				continue
			}
			passedSubcmds = true
			// Extract the canonical flag name: stop at the first character
			// that cannot appear in a flag name (letters, digits, hyphens).
			// This strips markdown artifacts like --json`，... → --json.
			name := extractFlagName(tok)
			if name == "-" || name == "--" {
				continue
			}
			flags = append(flags, name)
			continue
		}
		if passedSubcmds {
			// After first flag, remaining non-flag tokens are values — skip.
			continue
		}
		// Before any flag: collect only subcommand-shaped words.
		if subCmdWordRe.MatchString(tok) {
			words = append(words, tok)
		} else {
			passedSubcmds = true
		}
	}

	if len(words) == 0 {
		return skillExampleRef{}, false
	}
	return skillExampleRef{line: lineNo, raw: raw, words: words, flags: flags}, true
}

// ── helpers ───────────────────────────────────────────────────────────────────

// findSkillDir locates the Tier0-skill directory relative to this source file.
// It skips the test (with t.Skip) if the directory cannot be found, so the
// check is a no-op in environments where only the CLI repo is checked out.
func findSkillDir(t *testing.T) string {
	t.Helper()
	// Use the source file's location to navigate to the sibling Tier0-skill dir.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("runtime.Caller failed; skipping skill example check")
	}
	// thisFile = …/Tier0-cli/cmd/skillexample_test.go
	// Tier0-skill is at …/Tier0-skill/
	cliDir := filepath.Dir(filepath.Dir(thisFile)) // …/Tier0-cli
	skillDir := filepath.Join(filepath.Dir(cliDir), "Tier0-skill")
	if _, err := os.Stat(skillDir); err != nil {
		t.Skipf("Tier0-skill not found at %s; skipping skill example check", skillDir)
	}
	return skillDir
}
