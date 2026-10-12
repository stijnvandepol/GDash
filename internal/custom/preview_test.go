package custom

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPreviewCutsLongAnswers(t *testing.T) {
	small := decode(t, sampleAnswer)
	if got, cut := Preview(small); cut || !reflect.DeepEqual(got, small) {
		t.Fatalf("a small answer was changed: %v, %v", got, cut)
	}
	items := make([]any, 100)
	for i := range items {
		items[i] = strings.Repeat("x", 300)
	}
	got, cut := Preview(map[string]any{"items": items})
	list := got.(map[string]any)["items"].([]any)
	if !cut || len(list) != previewItems || len([]rune(list[0].(string))) != previewRunes+1 {
		t.Fatalf("Preview kept %d items, cut %v", len(list), cut)
	}
	var deep any = "bottom"
	for range previewDepth + 5 {
		deep = []any{deep}
	}
	if _, cut := Preview(deep); !cut {
		t.Fatal("a deep answer was not cut")
	}
}

func TestDocumentsDecodeOncePerAnswer(t *testing.T) {
	var docs Documents
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	first, err := docs.Decode("k", json.RawMessage(`{"a":1}`), at)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := docs.Decode("k", json.RawMessage(`{"a":1}`), at)
	if reflect.ValueOf(first).Pointer() != reflect.ValueOf(again).Pointer() {
		t.Error("the same answer was decoded twice")
	}
	newer, _ := docs.Decode("k", json.RawMessage(`{"a":2}`), at.Add(time.Minute))
	if newer.(map[string]any)["a"] != 2.0 {
		t.Errorf("a newer answer = %v", newer)
	}
	if _, err := docs.Decode("bad", json.RawMessage(`{`), at); err == nil {
		t.Error("invalid JSON decoded")
	}
}
