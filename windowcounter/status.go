package windowcounter

import (
	"math"
	"time"
)

// Status is the outcome of one Take or Peek: whether the hit is allowed, the sliding estimate, and the window values a caller
// needs to write rate-limit response headers. The library never writes HTTP. In buffered mode every field is this instance's
// view, the same one that decides Allowed.
type Status struct {
	// Allowed is true when Estimated is at most Limit. For Peek it is occupancy, not whether the next Take would admit.
	Allowed bool
	// Limit is the limit argument.
	Limit int64
	// Remaining is how many more Takes fit now: floor(max(Limit - Estimated, 0)).
	Remaining int64
	// Period is the window argument.
	Period time.Duration
	// Reset is the time from this call's clock reading to the end of the current epoch-aligned window. The previous window
	// still weighs at that instant, so it is not the time until the full limit is back.
	Reset time.Duration
	// RetryAfter is zero when allowed. When denied it is the time until the first whole second at which one more Take, with no
	// other traffic, is admitted, and it equals Reset when Limit is below 1. Denied Takes in between still count.
	RetryAfter time.Duration
	// Estimated is the sliding estimate current + previous x weight: after this hit for Take, without a hit for Peek.
	Estimated float64
}

// newStatus builds the Status of one Take or Peek from the counts its path read and the sliding window of the same call. The
// estimate and the admit comparison are the expressions Take and Peek have always used.
//
// The allowed path is one body on purpose: it runs on every Take and Peek, and under Yaegi each interpreted call costs about a
// microsecond (BenchmarkYaegiTake), so Remaining is computed inline. The retry-after search runs only on a deny.
func newStatus(sliding slidingWindow, limit, current, previous int64) Status {
	estimated := float64(current) + float64(previous)*sliding.weight
	status := Status{
		Allowed:   estimated <= float64(limit),
		Limit:     limit,
		Period:    time.Duration(sliding.windowSec) * time.Second,
		Reset:     time.Unix(sliding.windowStart+sliding.windowSec, 0).Sub(sliding.now),
		Estimated: estimated,
	}
	// Remaining is floor(max(limit - estimated, 0)), never above limit; int64 truncation is floor for headroom >= 1, and NaN
	// fails both comparisons and stays 0.
	headroom := float64(limit) - estimated
	if headroom >= 1 && headroom >= float64(limit) {
		status.Remaining = limit
	} else if headroom >= 1 {
		status.Remaining = int64(headroom)
	}
	if status.Allowed {
		return status
	}
	status.RetryAfter = retryAfterDeny(sliding, limit, current, previous, status.Reset)
	return status
}

// retryAfterDeny is the RetryAfter of a denied call: the time from its clock reading to the first whole second at which one
// more Take, with no other traffic, is admitted.
func retryAfterDeny(sliding slidingWindow, limit, current, previous int64, reset time.Duration) time.Duration {
	// No Take is ever admitted below a limit of 1; the window end keeps a client from retrying in a loop.
	if limit < 1 {
		return reset
	}
	retrySecond := sliding.windowStart + firstAdmitElapsed(sliding, limit, current, previous)
	return time.Unix(retrySecond, 0).Sub(sliding.now)
}

// firstAdmitElapsed is the first admitting second, counted from the current window start: in this window while the previous
// window's weight can still fall far enough, otherwise in the next window, where this window's count becomes previous and
// current starts at 0. It is always after the call's own second, because that call was denied.
func firstAdmitElapsed(sliding slidingWindow, limit, current, previous int64) int64 {
	windowSec := sliding.windowSec
	// Same window: only the previous term falls, so it needs room for one more hit and a previous count to wait out.
	callElapsed := sliding.now.Unix() - sliding.windowStart
	if current+1 <= limit && previous > 0 && callElapsed+1 < windowSec {
		candidate := closedFormElapsed(windowSec, limit-current-1, previous)
		if elapsed, ok := firstAdmittingSecond(current, previous, limit, windowSec, candidate, callElapsed+1, windowSec-1); ok {
			return elapsed
		}
	}
	// Next window: one hit plus this window's count weighted; at its end the weight is 0, so limit >= 1 always admits.
	candidate := closedFormElapsed(windowSec, limit-1, current)
	elapsed, _ := firstAdmittingSecond(0, current, limit, windowSec, candidate, 0, windowSec)
	return windowSec + elapsed
}

// closedFormElapsed is ceil(windowSec x (1 - headroom/previous)) clamped to [0, windowSec]: the first elapsed second at which
// previous hits, weighted by 1 - elapsed/window, fit in headroom. Float rounding can put it one second off;
// firstAdmittingSecond corrects it with the admission rule itself.
func closedFormElapsed(windowSec, headroom, previous int64) int64 {
	if previous <= 0 {
		return 0
	}
	elapsed := math.Ceil(float64(windowSec) * (1 - float64(headroom)/float64(previous)))
	if elapsed <= 0 {
		return 0
	}
	if elapsed >= float64(windowSec) {
		return windowSec
	}
	return int64(elapsed)
}

// firstAdmittingSecond is the smallest elapsed second in [low, high] at which admitsNextTake holds, starting from the
// closed-form candidate and stepping one second while float rounding disagrees. The rule only gets easier as elapsed grows
// (the weight falls), so stepping finds the boundary. ok is false when the rule fails even at high.
func firstAdmittingSecond(current, previous, limit, windowSec, candidate, low, high int64) (elapsed int64, ok bool) {
	elapsed = candidate
	if elapsed < low {
		elapsed = low
	}
	if elapsed > high {
		elapsed = high
	}
	// Step up while the candidate does not admit yet.
	for elapsed < high && !admitsNextTake(current, previous, limit, elapsed, windowSec) {
		elapsed++
	}
	if !admitsNextTake(current, previous, limit, elapsed, windowSec) {
		return 0, false
	}
	// Step down while the second before also admits.
	for elapsed > low && admitsNextTake(current, previous, limit, elapsed-1, windowSec) {
		elapsed--
	}
	return elapsed, true
}

// admitsNextTake is the admission rule for one more Take elapsedSec seconds into a window that holds current hits, after a
// window that holds previous hits: the estimate expression of newStatus with the weight function of slidingAt.
func admitsNextTake(current, previous, limit, elapsedSec, windowSec int64) bool {
	return float64(current+1)+float64(previous)*weightAt(elapsedSec, windowSec) <= float64(limit)
}
