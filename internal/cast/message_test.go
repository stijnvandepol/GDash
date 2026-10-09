package cast

import (
	"bytes"
	"strings"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	want := Message{
		SourceID:      "sender-0",
		DestinationID: "web-7",
		Namespace:     "urn:x-cast:app.nookboard.display",
		Payload:       `{"type":"snapshot","version":3}`,
	}
	var buf bytes.Buffer
	if err := writeFrame(&buf, want); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}
	got, err := readFrame(&buf)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestFrameRoundTripEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	want := Message{SourceID: "a", DestinationID: "b", Namespace: "c"}
	if err := writeFrame(&buf, want); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}
	got, err := readFrame(&buf)
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestWriteFrameRejectsOversizedMessage(t *testing.T) {
	m := Message{Payload: strings.Repeat("x", maxFrameSize)}
	if err := writeFrame(&bytes.Buffer{}, m); err == nil {
		t.Fatal("writeFrame accepted a message above the 64 KiB limit")
	}
}

func TestReadFrameRejectsOversizedHeader(t *testing.T) {
	frame := []byte{0x00, 0x02, 0x00, 0x00} // 128 KiB
	if _, err := readFrame(bytes.NewReader(frame)); err == nil {
		t.Fatal("readFrame accepted a frame above the 64 KiB limit")
	}
}

func TestUnmarshalRejectsTruncatedField(t *testing.T) {
	// Field 2 (source_id) claims 10 bytes but only 2 follow.
	body := []byte{0x12, 0x0a, 'a', 'b'}
	if _, err := unmarshalMessage(body); err == nil {
		t.Fatal("unmarshalMessage accepted a truncated field")
	}
}

func TestUnmarshalIgnoresBinaryPayload(t *testing.T) {
	m := Message{SourceID: "receiver-0", Namespace: "urn:x-cast:com.google.cast.tp.deviceauth"}
	body := appendBytesField(m.marshal(), fieldPayloadBinary, "\x00\x01")
	got, err := unmarshalMessage(body)
	if err != nil {
		t.Fatalf("unmarshalMessage: %v", err)
	}
	if got.SourceID != m.SourceID || got.Namespace != m.Namespace {
		t.Fatalf("got %+v, want source and namespace of %+v", got, m)
	}
}
