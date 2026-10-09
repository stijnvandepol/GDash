package display

import (
	"context"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func TestEachDisplayGetsOnlyItsOwnEntities(t *testing.T) {
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{entities("sensor.temp")}})
	living := Normalize(Dashboard{ID: "living", Widgets: []Widget{entities("sensor.temp", "lock.front")}})
	data := map[string]datahub.Entry{HomeAssistantSource: {Value: []byte(
		`{"sensor.temp":{"state":"21","name":"Temp"},"lock.front":{"state":"unlocked","name":"Front"}}`)}}
	b := NewBroadcaster()
	if err := b.Publish(now, map[string]View{"hall": {Dashboard: hall}, "living": {Dashboard: living}}, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[HomeAssistantSource].Value)
	if got != `{"sensor.temp":{"state":"21","name":"Temp"}}` {
		t.Fatalf("hall received %s, want only its temperature", got)
	}
	if got := decode(t, <-b.Subscribe(ctx, "living")).Data[HomeAssistantSource].Value; len(got) < 80 {
		t.Fatalf("living room received %s, want both entities", got)
	}
}

func TestUnreadableEntitiesAreDropped(t *testing.T) {
	entry := onlyEntities(datahub.Entry{Value: []byte(`[1,2]`), Failing: true}, []string{"sensor.temp"})
	if entry.Value != nil || !entry.Failing {
		t.Fatalf("onlyEntities = %+v, want no value and the status kept", entry)
	}
}
