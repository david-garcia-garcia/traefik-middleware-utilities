//go:build race

package simpleredis

// raceDetectorOn is true in race builds so alloc ceilings skip inflated B/op.
const raceDetectorOn = true
