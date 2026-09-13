//go:build !race

package reclaim

// raceDetectorOn is false in non-race builds so Yaegi hook tests still run.
const raceDetectorOn = false
