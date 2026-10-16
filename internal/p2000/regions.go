package p2000

import "slices"

// Region is a safety region (veiligheidsregio).
type Region struct {
	// Slug is the region in alarmeringen.nl's feed addresses and links.
	Slug string
	Name string
}

// regions are the 25 safety regions, each checked against its feed on
// 16 October 2026. Two slugs differ from the region's own name: friesland
// for Fryslân and limburg-zuid for Zuid-Limburg.
var regions = []Region{
	{"groningen", "Groningen"},
	{"friesland", "Fryslân"},
	{"drenthe", "Drenthe"},
	{"ijsselland", "IJsselland"},
	{"twente", "Twente"},
	{"noord-en-oost-gelderland", "Noord- en Oost-Gelderland"},
	{"gelderland-midden", "Gelderland-Midden"},
	{"gelderland-zuid", "Gelderland-Zuid"},
	{"utrecht", "Utrecht"},
	{"noord-holland-noord", "Noord-Holland Noord"},
	{"zaanstreek-waterland", "Zaanstreek-Waterland"},
	{"kennemerland", "Kennemerland"},
	{"amsterdam-amstelland", "Amsterdam-Amstelland"},
	{"gooi-en-vechtstreek", "Gooi en Vechtstreek"},
	{"haaglanden", "Haaglanden"},
	{"hollands-midden", "Hollands Midden"},
	{"rotterdam-rijnmond", "Rotterdam-Rijnmond"},
	{"zuid-holland-zuid", "Zuid-Holland Zuid"},
	{"zeeland", "Zeeland"},
	{"midden-en-west-brabant", "Midden- en West-Brabant"},
	{"brabant-noord", "Brabant-Noord"},
	{"brabant-zuidoost", "Brabant-Zuidoost"},
	{"limburg-noord", "Limburg-Noord"},
	{"limburg-zuid", "Zuid-Limburg"},
	{"flevoland", "Flevoland"},
}

// RegionSlugs are the slugs a widget's region setting takes.
var RegionSlugs = func() []string {
	slugs := make([]string, len(regions))
	for i, r := range regions {
		slugs[i] = r.Slug
	}
	return slugs
}()

// ValidRegion reports whether slug is one of RegionSlugs.
func ValidRegion(slug string) bool { return slices.Contains(RegionSlugs, slug) }

// RegionName returns the name of the region with slug, or "".
func RegionName(slug string) string {
	for _, r := range regions {
		if r.Slug == slug {
			return r.Name
		}
	}
	return ""
}
