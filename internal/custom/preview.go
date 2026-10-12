package custom

import (
	"maps"
	"slices"
	"unicode/utf8"
)

// Limits of the answer that the editor shows as a tree. A path that is
// cut off can still be typed; the tree is for finding one.
const (
	previewItems = 25
	previewRunes = 200
	previewDepth = 12
	previewNodes = 2000
)

// Preview cuts an answer down for the editor: the first items of each
// list, the start of long text, a bounded depth and number of values. cut
// reports whether anything was left out.
func Preview(doc any) (value any, cut bool) {
	budget := previewNodes
	value = previewValue(doc, 0, &budget, &cut)
	return value, cut
}

func previewValue(v any, depth int, budget *int, cut *bool) any {
	*budget--
	switch x := v.(type) {
	case string:
		if utf8.RuneCountInString(x) > previewRunes {
			*cut = true
			return string([]rune(x)[:previewRunes]) + "…"
		}
		return x
	case []any:
		if depth >= previewDepth {
			*cut = true
			return []any{}
		}
		out := []any{}
		for i, item := range x {
			if i == previewItems || *budget <= 0 {
				*cut = true
				break
			}
			out = append(out, previewValue(item, depth+1, budget, cut))
		}
		return out
	case map[string]any:
		if depth >= previewDepth {
			*cut = true
			return map[string]any{}
		}
		out := map[string]any{}
		// Sorted, so the same answer is always cut in the same place.
		for _, key := range slices.Sorted(maps.Keys(x)) {
			if *budget <= 0 {
				*cut = true
				break
			}
			out[key] = previewValue(x[key], depth+1, budget, cut)
		}
		return out
	}
	return v
}
