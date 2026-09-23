// Package locales embeds the go-i18n message files so every binary built from
// this module (bot, API, Galactus) carries the same set of languages. Nothing
// needs to be copied into an image or pointed at with an environment variable.
//
// active.en.toml is the source; the other files come from Crowdin. A new
// translation is picked up by the next build with no code change.
package locales

import "embed"

// FS holds the active.<tag>.toml message files.
//
//go:embed active.*.toml
var FS embed.FS
