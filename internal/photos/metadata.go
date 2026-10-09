package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// errMalformed means a photo's container could not be walked; such a file
// is refused rather than stored with metadata that could not be removed.
var errMalformed = errors.New("malformed image container")

// stripMetadata removes what a photo says about where, when and with what
// it was taken: EXIF (with GPS), XMP, IPTC, comments and text chunks. A
// phone photo carries the home's location there, and photos go to every
// paired screen. Only the JPEG orientation survives, as a minimal EXIF
// block, so portrait photos keep standing; colour profiles stay too. The
// image data itself is copied unchanged, so nothing is re-encoded.
func stripMetadata(ext string, data []byte) ([]byte, error) {
	switch ext {
	case ".jpg":
		return stripJPEG(data)
	case ".png":
		return stripPNG(data)
	case ".webp":
		return stripWebP(data)
	}
	return nil, errMalformed
}

const (
	markerSOS  = 0xDA // start of scan: entropy-coded data follows
	markerAPP0 = 0xE0 // JFIF
	markerAPP1 = 0xE1 // EXIF, XMP
	markerAPP2 = 0xE2 // ICC colour profile, among others
	markerAPPE = 0xEE // Adobe: needed to decode CMYK and YCCK correctly
	markerAPPF = 0xEF
	markerCOM  = 0xFE
)

func stripJPEG(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, errMalformed
	}
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(data[:2])
	orientation := uint16(0)
	wroteOrientation := false
	for i := 2; ; {
		if i+4 > len(data) || data[i] != 0xFF {
			return nil, errMalformed
		}
		marker := data[i+1]
		if marker == 0xFF { // fill byte
			i++
			continue
		}
		if marker == markerSOS {
			out.Write(data[i:])
			return out.Bytes(), nil
		}
		length := int(binary.BigEndian.Uint16(data[i+2:]))
		end := i + 2 + length
		if length < 2 || end > len(data) {
			return nil, errMalformed
		}
		segment, payload := data[i:end], data[i+4:end]
		switch {
		case marker == markerAPP1:
			if o := exifOrientation(payload); o != 0 {
				orientation = o
			}
		case marker == markerCOM, marker >= markerAPP0+1 && marker <= markerAPPF && marker != markerAPP2 && marker != markerAPPE:
			// Dropped: comments and APP segments other than JFIF, ICC and Adobe.
		case marker == markerAPP2 && !bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00")):
			// Dropped: APP2 that is not a colour profile.
		default:
			// The orientation goes right after JFIF, before the frame header.
			if !wroteOrientation && marker != markerAPP0 && orientation > 1 {
				out.Write(orientationSegment(orientation))
				wroteOrientation = true
			}
			out.Write(segment)
		}
		i = end
	}
}

// exifOrientation reads the Orientation tag (0x0112) from IFD0 of an EXIF
// APP1 payload, or returns 0.
func exifOrientation(p []byte) uint16 {
	if !bytes.HasPrefix(p, []byte("Exif\x00\x00")) || len(p) < 14 {
		return 0
	}
	tiff := p[6:]
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	ifd := int(order.Uint32(tiff[4:]))
	if ifd+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[ifd:]))
	for e := 0; e < count; e++ {
		at := ifd + 2 + 12*e
		if at+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[at:]) == 0x0112 && order.Uint16(tiff[at+2:]) == 3 {
			if v := order.Uint16(tiff[at+8:]); v >= 1 && v <= 8 {
				return v
			}
		}
	}
	return 0
}

// orientationSegment is an APP1 EXIF segment holding only the orientation.
func orientationSegment(orientation uint16) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, markerAPP1, 0, 0})
	b.WriteString("Exif\x00\x00")
	b.WriteString("MM\x00\x2A\x00\x00\x00\x08") // big-endian TIFF, IFD0 at 8
	b.Write([]byte{0, 1})                       // one entry
	b.Write([]byte{0x01, 0x12, 0, 3, 0, 0, 0, 1})
	b.Write([]byte{byte(orientation >> 8), byte(orientation), 0, 0})
	b.Write([]byte{0, 0, 0, 0}) // no next IFD
	seg := b.Bytes()
	binary.BigEndian.PutUint16(seg[2:], uint16(len(seg)-2))
	return seg
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// pngDropped are the chunks that carry metadata rather than the image.
var pngDropped = map[string]bool{"eXIf": true, "tEXt": true, "zTXt": true, "iTXt": true, "tIME": true}

func stripPNG(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, errMalformed
	}
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(pngSignature)
	for i := len(pngSignature); i < len(data); {
		if i+8 > len(data) {
			return nil, errMalformed
		}
		length := int(binary.BigEndian.Uint32(data[i:]))
		end := i + 12 + length // length, type, data, CRC
		if length < 0 || end > len(data) {
			return nil, errMalformed
		}
		if kind := string(data[i+4 : i+8]); !pngDropped[kind] {
			out.Write(data[i:end])
		}
		i = end
	}
	return out.Bytes(), nil
}

// VP8X flags: bit 3 says an EXIF chunk follows, bit 2 an XMP chunk.
const (
	webpEXIFFlag = 0x08
	webpXMPFlag  = 0x04
)

func stripWebP(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errMalformed
	}
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(data[:12])
	for i := 12; i < len(data); {
		if i+8 > len(data) {
			return nil, errMalformed
		}
		size := int(binary.LittleEndian.Uint32(data[i+4:]))
		end := i + 8 + size + size%2 // chunks are padded to an even size
		if size < 0 || end > len(data) {
			return nil, errMalformed
		}
		switch kind := string(data[i : i+4]); kind {
		case "EXIF", "XMP ":
		case "VP8X":
			chunk := append([]byte(nil), data[i:end]...)
			if len(chunk) > 8 {
				chunk[8] &^= webpEXIFFlag | webpXMPFlag
			}
			out.Write(chunk)
		default:
			out.Write(data[i:end])
		}
		i = end
	}
	result := out.Bytes()
	binary.LittleEndian.PutUint32(result[4:], uint32(len(result)-8))
	return result, nil
}
