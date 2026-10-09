//go:build !race

package calendar

// raceSlowdown widens time budgets in tests under the race detector; see
// race_test.go.
const raceSlowdown = 1
