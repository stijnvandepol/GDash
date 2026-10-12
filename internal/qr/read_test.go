package qr

import "testing"

// readBack reads a code the way a scanner does once it has found the grid:
// format information, unmasking, the zigzag, de-interleaving, a
// Reed-Solomon check of every block, and the byte-mode segment. It reuses
// the encoder's map of function modules, but reads with its own loops, so
// a mistake in placement or interleaving shows as a mismatch.
func readBack(t *testing.T, c *Code) string {
	t.Helper()
	mask := readMask(t, c)
	layout := newMatrix(c.Version)
	layout.drawFunctionPatterns(c.Version)

	var bits []bool
	for right := c.Size - 1; right > 0; right -= 2 {
		if right == 6 {
			right--
		}
		upward := (c.Size-1-right)/2%2 == 0
		if right < 6 {
			upward = (c.Size-1-(right+1))/2%2 == 0
		}
		for i := range c.Size {
			y := i
			if upward {
				y = c.Size - 1 - i
			}
			for _, x := range []int{right, right - 1} {
				if !layout.reserved[y][x] {
					bits = append(bits, c.Dark(x, y) != masks[mask](x, y))
				}
			}
		}
	}
	v := versions[c.Version-1]
	words := make([]byte, len(bits)/8)
	for i := range words {
		for j := range 8 {
			if bits[i*8+j] {
				words[i] |= 1 << (7 - j)
			}
		}
	}

	blocks := v.group1Blocks + v.group2Blocks
	data := make([][]byte, blocks)
	ecc := make([][]byte, blocks)
	next := 0
	for i := range max(v.group1Data, v.group2Data) {
		for b := range blocks {
			if size := blockSize(v, b); i < size {
				data[b] = append(data[b], words[next])
				next++
			}
		}
	}
	for range v.ecPerBlock {
		for b := range blocks {
			ecc[b] = append(ecc[b], words[next])
			next++
		}
	}
	var stream []byte
	for b := range blocks {
		if !syndromesAreZero(append(append([]byte{}, data[b]...), ecc[b]...), v.ecPerBlock) {
			t.Fatalf("version %d block %d fails its Reed-Solomon check", c.Version, b)
		}
		stream = append(stream, data[b]...)
	}
	return readSegment(t, stream, c.Version)
}

func blockSize(v version, block int) int {
	if block < v.group1Blocks {
		return v.group1Data
	}
	return v.group2Data
}

// readMask finds the mask from both copies of the format information.
func readMask(t *testing.T, c *Code) int {
	t.Helper()
	var first, second int
	for i := range 15 {
		var x1, y1, x2, y2 int
		switch {
		case i <= 5:
			x1, y1 = 8, i
		case i == 6:
			x1, y1 = 8, 7
		case i == 7:
			x1, y1 = 8, 8
		case i == 8:
			x1, y1 = 7, 8
		default:
			x1, y1 = 14-i, 8
		}
		if i < 8 {
			x2, y2 = c.Size-1-i, 8
		} else {
			x2, y2 = 8, c.Size-15+i
		}
		if c.Dark(x1, y1) {
			first |= 1 << i
		}
		if c.Dark(x2, y2) {
			second |= 1 << i
		}
	}
	if first != second {
		t.Fatalf("format copies differ: %015b and %015b", first, second)
	}
	for mask := range masks {
		if formatBits(mask) == first {
			return mask
		}
	}
	t.Fatalf("format information %015b is not level M with a known mask", first)
	return 0
}

// syndromesAreZero evaluates the codeword polynomial at α^0 … α^(n-1); all
// are zero for a block without errors.
func syndromesAreZero(block []byte, n int) bool {
	root := byte(1)
	for range n {
		var sum byte
		for _, b := range block {
			sum = gfMultiply(sum, root) ^ b
		}
		if sum != 0 {
			return false
		}
		root = gfMultiply(root, 2)
	}
	return true
}

func readSegment(t *testing.T, stream []byte, version int) string {
	t.Helper()
	pos := 0
	read := func(n int) int {
		value := 0
		for range n {
			value = value<<1 | int(stream[pos/8]>>(7-pos%8)&1)
			pos++
		}
		return value
	}
	if mode := read(4); mode != 0b0100 {
		t.Fatalf("mode %04b, want byte mode", mode)
	}
	length := read(countBits(version))
	out := make([]byte, length)
	for i := range out {
		out[i] = byte(read(8))
	}
	return string(out)
}
