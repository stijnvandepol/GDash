package display

// Themes are the built-in themes; web/src/themes.css defines their colours,
// shapes and textures. A dashboard file names its theme, so a theme keeps
// its name once it ships.
var Themes = []string{
	"minimal-dark", "minimal-light", "glass", "warm-light", "high-contrast", "night",
	"frost", "walnut", "swiss", "tonal", "gilded", "riviera", "galaxy", "terminal", "parchment", "neon",
	"frost-night", "candy", "upside-down", "noir", "mars",
	// Film, series and game presets (web/src/presets-film.css).
	"hyperspace", "dark-side", "desert-planet", "wizard-school", "middle-realm", "rain-city", "time-machine",
	"secret-agent", "shiny-hunt", "pocket-console", "block-world", "quest-menu",
}

// Fonts are the typefaces a dashboard can choose instead of its theme's own;
// web/src/fonts.css loads them. Empty means the theme's font.
var Fonts = []string{
	"geist", "inter", "outfit", "space-grotesk", "lora", "cormorant", "bodoni", "news-cycle", "jetbrains-mono", "im-fell",
	"courier-prime",
	"oswald", "jersey",
	"jost", "fraunces",
}
