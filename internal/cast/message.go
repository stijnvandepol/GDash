// Package cast talks to Google Cast devices on the local network: it finds
// them with mDNS and controls them over the CASTV2 protocol, without any
// Google cloud service in between.
package cast

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// maxFrameSize is the largest CASTV2 message a device accepts (64 KiB).
const maxFrameSize = 64 * 1024

// Message is one CASTV2 CastMessage. Only UTF-8 payloads are supported,
// because every namespace GDash uses exchanges JSON.
type Message struct {
	SourceID      string
	DestinationID string
	Namespace     string
	Payload       string
}

// Protobuf field numbers of the CastMessage definition in Chromium's
// cast_channel.proto. Encoding by hand avoids a protobuf dependency for a
// message with seven fields.
const (
	fieldProtocolVersion = 1
	fieldSourceID        = 2
	fieldDestinationID   = 3
	fieldNamespace       = 4
	fieldPayloadType     = 5
	fieldPayloadUTF8     = 6
	fieldPayloadBinary   = 7

	wireVarint = 0
	wireBytes  = 2
)

func (m Message) marshal() []byte {
	b := make([]byte, 0, 32+len(m.SourceID)+len(m.DestinationID)+len(m.Namespace)+len(m.Payload))
	// protocol_version and payload_type are proto2 "required" fields, so they
	// are written even though their value (CASTV2_1_0, STRING) is zero.
	b = appendVarintField(b, fieldProtocolVersion, 0)
	b = appendBytesField(b, fieldSourceID, m.SourceID)
	b = appendBytesField(b, fieldDestinationID, m.DestinationID)
	b = appendBytesField(b, fieldNamespace, m.Namespace)
	b = appendVarintField(b, fieldPayloadType, 0)
	b = appendBytesField(b, fieldPayloadUTF8, m.Payload)
	return b
}

func appendVarintField(b []byte, field int, v uint64) []byte {
	b = binary.AppendUvarint(b, uint64(field<<3|wireVarint))
	return binary.AppendUvarint(b, v)
}

func appendBytesField(b []byte, field int, s string) []byte {
	b = binary.AppendUvarint(b, uint64(field<<3|wireBytes))
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

var errMalformed = errors.New("malformed cast message")

func unmarshalMessage(b []byte) (Message, error) {
	var m Message
	for len(b) > 0 {
		key, n := binary.Uvarint(b)
		if n <= 0 {
			return Message{}, errMalformed
		}
		b = b[n:]
		field, wire := int(key>>3), int(key&7)
		switch wire {
		case wireVarint:
			_, n = binary.Uvarint(b)
			if n <= 0 {
				return Message{}, errMalformed
			}
			b = b[n:]
		case wireBytes:
			size, n := binary.Uvarint(b)
			if n <= 0 || uint64(len(b)-n) < size {
				return Message{}, errMalformed
			}
			value := string(b[n : n+int(size)])
			b = b[n+int(size):]
			switch field {
			case fieldSourceID:
				m.SourceID = value
			case fieldDestinationID:
				m.DestinationID = value
			case fieldNamespace:
				m.Namespace = value
			case fieldPayloadUTF8:
				m.Payload = value
			case fieldPayloadBinary:
				// Binary payloads are only used for device authentication,
				// which GDash does not perform.
			}
		default:
			return Message{}, fmt.Errorf("%w: unsupported wire type %d", errMalformed, wire)
		}
	}
	return m, nil
}

// writeFrame writes a message with the 4-byte big-endian length prefix that
// CASTV2 uses on the wire.
func writeFrame(w io.Writer, m Message) error {
	body := m.marshal()
	if len(body) > maxFrameSize {
		return fmt.Errorf("cast message of %d bytes exceeds the %d byte limit", len(body), maxFrameSize)
	}
	frame := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(body)), uint32(len(body)))
	_, err := w.Write(append(frame, body...))
	return err
}

func readFrame(r io.Reader) (Message, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Message{}, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size > maxFrameSize {
		return Message{}, fmt.Errorf("%w: frame of %d bytes", errMalformed, size)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return Message{}, err
	}
	return unmarshalMessage(body)
}
