//go:build race

package reclaim

// raceDetectorOn is true in race builds so tests can skip Yaegi interp races.
const raceDetectorOn = true
