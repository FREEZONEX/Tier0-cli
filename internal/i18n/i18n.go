// Package i18n provides minimal bilingual (English/Chinese) support.
// The active language is set once at startup via SetLang and then
// read-only for the lifetime of the process.
package i18n

import "strings"

// Lang represents a supported UI language.
type Lang string

const (
	EN Lang = "en"
	ZH Lang = "zh"
)

var current Lang = EN

// SetLang sets the active language. Accepts "en", "zh" (case-insensitive).
// Unknown values fall back to EN.
func SetLang(lang string) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh", "zh-cn", "zh_cn":
		current = ZH
	default:
		current = EN
	}
}

// Current returns the active language.
func Current() Lang { return current }

// T returns the English string when the active language is EN,
// and the Chinese string when it is ZH.
func T(en, zh string) string {
	if current == ZH {
		return zh
	}
	return en
}
