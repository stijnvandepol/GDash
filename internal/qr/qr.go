// Package qr draws QR codes (ISO/IEC 18004) for short text, such as the
// otpauth:// address an authenticator app reads. It knows only what that
// needs: byte mode, error correction level M and versions 1 to 10 (up to
// 213 bytes). A library would bring all of QR for one small image.
package qr

import (
	"errors"
	"fmt"
	"strings"
)

// MaxBytes is the most data a code of version 10 at level M holds.
const MaxBytes = 213

// quietZone is the light border around a code that scanners need, in
// modules.
const quietZone = 4

// ErrTooLong is returned for data above MaxBytes.
var ErrTooLong = errors.New("data is too long for a QR code")

// version describes one QR version at error correction level M: its
// blocks of data codewords, the error correction codewords per block, and
// the centres of its alignment patterns.
type version struct {
	ecPerBlock   int
	group1Blocks int
	group1Data   int
	group2Blocks int
	group2Data   int
	align        []int
}

// versions holds versions 1 to 10 at level M, from ISO/IEC 18004 tables 9
// and E.1.
var versions = [...]version{
	{10, 1, 16, 0, 0, nil},
	{16, 1, 28, 0, 0, []int{6, 18}},
	{26, 1, 44, 0, 0, []int{6, 22}},
	{18, 2, 32, 0, 0, []int{6, 26}},
	{24, 2, 43, 0, 0, []int{6, 30}},
	{16, 4, 27, 0, 0, []int{6, 34}},
	{18, 4, 31, 0, 0, []int{6, 22, 38}},
	{22, 2, 38, 2, 39, []int{6, 24, 42}},
	{22, 3, 36, 2, 37, []int{6, 26, 46}},
	{26, 4, 43, 1, 44, []int{6, 28, 50}},
}

func (v version) dataCodewords() int {
	return v.group1Blocks*v.group1Data + v.group2Blocks*v.group2Data
}

// countBits is the length of the byte-mode character count for version n.
func countBits(n int) int {
	if n <= 9 {
		return 8
	}
	return 16
}

// capacity is how many bytes version n holds in byte mode.
func capacity(n int) int {
	return (versions[n-1].dataCodewords()*8 - 4 - countBits(n)) / 8
}

// Code is a QR code: a square of dark and light modules.
type Code struct {
	// Version is 1 to 10; the code is 17 + 4 × Version modules wide.
	Version int
	Size    int
	dark    [][]bool
}

// Dark reports whether the module in column x, row y is dark.
func (c *Code) Dark(x, y int) bool { return c.dark[y][x] }

// Encode returns the smallest code that holds data.
func Encode(data []byte) (*Code, error) {
	n := 1
	for n <= len(versions) && capacity(n) < len(data) {
		n++
	}
	if n > len(versions) {
		return nil, fmt.Errorf("%w: %d bytes, at most %d", ErrTooLong, len(data), MaxBytes)
	}
	m := newMatrix(n)
	m.drawFunctionPatterns(n)
	m.drawCodewords(interleave(versions[n-1], dataCodewords(data, n)))
	mask := m.chooseMask()
	m.applyMask(mask)
	m.drawFormat(mask)
	return &Code{Version: n, Size: m.size, dark: m.dark}, nil
}

// dataCodewords is the bit stream of data in byte mode, ended and padded to
// the data capacity of version n.
func dataCodewords(data []byte, n int) []byte {
	total := versions[n-1].dataCodewords() * 8
	var b bitBuffer
	b.append(0b0100, 4) // byte mode
	b.append(len(data), countBits(n))
	for _, d := range data {
		b.append(int(d), 8)
	}
	b.append(0, min(4, total-b.len())) // terminator
	b.append(0, (8-b.len()%8)%8)
	for pad := 0xEC; b.len() < total; pad ^= 0xEC ^ 0x11 {
		b.append(pad, 8)
	}
	return b.bytes()
}

// bitBuffer collects bits, most significant first.
type bitBuffer struct{ bits []bool }

func (b *bitBuffer) append(value, length int) {
	for i := length - 1; i >= 0; i-- {
		b.bits = append(b.bits, value>>i&1 == 1)
	}
}

func (b *bitBuffer) len() int { return len(b.bits) }

func (b *bitBuffer) bytes() []byte {
	out := make([]byte, len(b.bits)/8)
	for i, bit := range b.bits {
		if bit {
			out[i/8] |= 1 << (7 - i%8)
		}
	}
	return out
}

// interleave splits data into the blocks of v, adds each block's error
// correction, and interleaves them as the standard places them: the first
// codeword of every block, then the second, and so on.
func interleave(v version, data []byte) []byte {
	divisor := rsDivisor(v.ecPerBlock)
	var blocks, ecc [][]byte
	for i := range v.group1Blocks + v.group2Blocks {
		size := v.group1Data
		if i >= v.group1Blocks {
			size = v.group2Data
		}
		blocks = append(blocks, data[:size])
		ecc = append(ecc, rsRemainder(data[:size], divisor))
		data = data[size:]
	}
	var out []byte
	for i := range max(v.group1Data, v.group2Data) {
		for _, block := range blocks {
			if i < len(block) {
				out = append(out, block[i])
			}
		}
	}
	for i := range v.ecPerBlock {
		for _, e := range ecc {
			out = append(out, e[i])
		}
	}
	return out
}

// SVG returns the code as an SVG image with its quiet zone, one unit per
// module. It is always black on white, whatever the page's theme: not
// every scanner reads a light code on a dark ground.
func (c *Code) SVG() string {
	n := c.Size + 2*quietZone
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := range c.Size {
		for x := 0; x < c.Size; x++ {
			if !c.dark[y][x] {
				continue
			}
			run := 1
			for x+run < c.Size && c.dark[y][x+run] {
				run++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x+quietZone, y+quietZone, run, run)
			x += run - 1
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}
