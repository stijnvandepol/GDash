package qr

// Penalty weights of ISO/IEC 18004, section 7.8.3.
const (
	penaltyRun     = 3  // N1, a run of five; one more per longer module
	penaltyBlock   = 3  // N2, per 2×2 block of one colour
	penaltyFinder  = 40 // N3, per pattern that looks like a finder
	penaltyBalance = 10 // N4, per 5 % that dark modules are off 50 %
)

// finderLike is 1:1:3:1:1 with four light modules on one side, the pattern
// N3 counts in either direction.
var finderLike = [2][11]bool{
	{true, false, true, true, true, false, true, false, false, false, false},
	{false, false, false, false, true, false, true, true, true, false, true},
}

// penalty scores the matrix as it stands: lower is easier to scan.
func (m *matrix) penalty() int {
	total := 0
	for i := range m.size {
		row := make([]bool, m.size)
		col := make([]bool, m.size)
		for j := range m.size {
			row[j], col[j] = m.dark[i][j], m.dark[j][i]
		}
		total += linePenalty(row) + linePenalty(col)
	}
	dark := 0
	for y := range m.size {
		for x := range m.size {
			if m.dark[y][x] {
				dark++
			}
			if x+1 < m.size && y+1 < m.size {
				c := m.dark[y][x]
				if m.dark[y][x+1] == c && m.dark[y+1][x] == c && m.dark[y+1][x+1] == c {
					total += penaltyBlock
				}
			}
		}
	}
	modules := m.size * m.size
	total += penaltyBalance * (abs(dark*20-modules*10) / modules)
	return total
}

// linePenalty scores one row or column for N1 and N3. The quiet zone around
// the code is light, so the line is read with four light modules on each
// side.
func linePenalty(line []bool) int {
	total := 0
	for start := 0; start < len(line); {
		end := start
		for end < len(line) && line[end] == line[start] {
			end++
		}
		if run := end - start; run >= 5 {
			total += penaltyRun + run - 5
		}
		start = end
	}
	padded := make([]bool, len(line)+2*quietZone)
	copy(padded[quietZone:], line)
	for start := 0; start+11 <= len(padded); start++ {
		for _, pattern := range finderLike {
			if matches(padded[start:start+11], pattern[:]) {
				total += penaltyFinder
			}
		}
	}
	return total
}

func matches(a, b []bool) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
