package display

// Themes are the built-in themes; web/src/themes.css defines their colours,
// shapes and textures. A dashboard file names its theme, so a theme keeps
// its name once it ships.
var Themes = []string{
	"minimal-dark", "minimal-light", "glass", "warm-light", "high-contrast", "night",
	"frost", "walnut", "swiss", "tonal", "gilded", "riviera", "galaxy", "terminal", "parchment", "neon",
}

// Fonts are the typefaces a dashboard can choose instead of its theme's own;
// web/src/fonts.css loads them. Empty means the theme's font.
var Fonts = []string{
	"geist", "inter", "outfit", "space-grotesk", "lora", "cormorant", "bodoni", "news-cycle", "jetbrains-mono", "im-fell",
}
