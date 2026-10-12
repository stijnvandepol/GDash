package qr

// matrix is a code being drawn. Coordinates are x (column) and y (row), as
// in the standard. Function modules (finders, timing, alignment, format and
// version information) are marked reserved, so data and masks skip them.
type matrix struct {
	size     int
	dark     [][]bool
	reserved [][]bool
}

func newMatrix(version int) *matrix {
	size := 17 + 4*version
	m := &matrix{size: size, dark: make([][]bool, size), reserved: make([][]bool, size)}
	for y := range size {
		m.dark[y] = make([]bool, size)
		m.reserved[y] = make([]bool, size)
	}
	return m
}

func (m *matrix) set(x, y int, dark bool) {
	m.dark[y][x] = dark
	m.reserved[y][x] = true
}

func (m *matrix) drawFunctionPatterns(version int) {
	for i := range m.size {
		m.set(6, i, i%2 == 0)
		m.set(i, 6, i%2 == 0)
	}
	m.drawFinder(3, 3)
	m.drawFinder(m.size-4, 3)
	m.drawFinder(3, m.size-4)
	align := versions[version-1].align
	last := len(align) - 1
	for i, ay := range align {
		for j, ax := range align {
			// The three corners hold finder patterns instead.
			if i == 0 && j == 0 || i == 0 && j == last || i == last && j == 0 {
				continue
			}
			m.drawAlignment(ax, ay)
		}
	}
	// Reserves the format areas; Encode draws them again with the mask.
	m.drawFormat(0)
	m.drawVersion(version)
}

// drawFinder draws a finder pattern centred on x, y with its light
// separator, cut off at the edges of the code.
func (m *matrix) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= m.size || y >= m.size {
				continue
			}
			ring := max(abs(dx), abs(dy))
			m.set(x, y, ring != 2 && ring != 4)
		}
	}
}

func (m *matrix) drawAlignment(cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			m.set(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

// formatBits is the 15-bit format information for level M and mask: a
// BCH(15,5) code, XORed with 101010000010010 so it is never all light.
func formatBits(mask int) int {
	const levelM = 0b00
	data := levelM<<3 | mask
	rem := data
	for range 10 {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	return (data<<10 | rem) ^ 0x5412
}

// drawFormat draws both copies of the format information and the dark
// module next to the lower left finder.
func (m *matrix) drawFormat(mask int) {
	bits := formatBits(mask)
	bit := func(i int) bool { return bits>>i&1 == 1 }
	for i := 0; i <= 5; i++ {
		m.set(8, i, bit(i))
	}
	m.set(8, 7, bit(6))
	m.set(8, 8, bit(7))
	m.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		m.set(14-i, 8, bit(i))
	}
	for i := range 8 {
		m.set(m.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		m.set(8, m.size-15+i, bit(i))
	}
	m.set(8, m.size-8, true)
}

// versionBits is the 18-bit version information: a BCH(18,6) code.
func versionBits(version int) int {
	rem := version
	for range 12 {
		rem = rem<<1 ^ (rem>>11)*0x1F25
	}
	return version<<12 | rem
}

// drawVersion draws both copies of the version information, which only
// versions 7 and up carry.
func (m *matrix) drawVersion(version int) {
	if version < 7 {
		return
	}
	bits := versionBits(version)
	for i := range 18 {
		dark := bits>>i&1 == 1
		a, b := m.size-11+i%3, i/3
		m.set(a, b, dark)
		m.set(b, a, dark)
	}
}

// drawCodewords places data in the zigzag order of the standard: two
// columns at a time from the right, up and down in turn, skipping the
// vertical timing pattern and every function module. Remainder modules
// stay light.
func (m *matrix) drawCodewords(data []byte) {
	i := 0
	for right := m.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		upward := (right+1)&2 == 0
		for vert := range m.size {
			y := vert
			if upward {
				y = m.size - 1 - vert
			}
			for j := range 2 {
				x := right - j
				if m.reserved[y][x] || i >= len(data)*8 {
					continue
				}
				m.dark[y][x] = data[i/8]>>(7-i%8)&1 == 1
				i++
			}
		}
	}
}

// masks are the eight data mask patterns; a module is flipped where the
// pattern is true.
var masks = [8]func(x, y int) bool{
	func(x, y int) bool { return (x+y)%2 == 0 },
	func(_, y int) bool { return y%2 == 0 },
	func(x, _ int) bool { return x%3 == 0 },
	func(x, y int) bool { return (x+y)%3 == 0 },
	func(x, y int) bool { return (x/3+y/2)%2 == 0 },
	func(x, y int) bool { return x*y%2+x*y%3 == 0 },
	func(x, y int) bool { return (x*y%2+x*y%3)%2 == 0 },
	func(x, y int) bool { return ((x+y)%2+x*y%3)%2 == 0 },
}

// applyMask flips the data modules where mask is true; applying it twice
// undoes it.
func (m *matrix) applyMask(mask int) {
	for y := range m.size {
		for x := range m.size {
			if !m.reserved[y][x] && masks[mask](x, y) {
				m.dark[y][x] = !m.dark[y][x]
			}
		}
	}
}

// chooseMask returns the mask with the lowest penalty, as the standard
// asks: the one that leaves the fewest patterns that confuse a scanner.
func (m *matrix) chooseMask() int {
	best, bestPenalty := 0, -1
	for mask := range masks {
		m.applyMask(mask)
		m.drawFormat(mask)
		if p := m.penalty(); bestPenalty < 0 || p < bestPenalty {
			best, bestPenalty = mask, p
		}
		m.applyMask(mask)
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
