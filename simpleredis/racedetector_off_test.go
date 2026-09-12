//go:build !race

package simpleredis

// raceDetectorOn is false in non-race builds so alloc ceilings still run.
const raceDetectorOn = false
