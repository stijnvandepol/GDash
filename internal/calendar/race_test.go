//go:build race

package calendar

// raceSlowdown widens time budgets in tests: the race detector makes code
// several times slower, which says nothing about its speed in production.
const raceSlowdown = 20
