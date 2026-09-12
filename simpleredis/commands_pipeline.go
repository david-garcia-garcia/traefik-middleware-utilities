package simpleredis

import "time"

const (
	// maxPipelineCommands is the ExecPipeline cap. Empty does not dial; over this is redis:issue? without send.
	maxPipelineCommands = 64
)

// PipelineSlot is one command's reply inside an ExecPipeline batch.
type PipelineSlot struct {
	Values [][]byte
	Err    error
}

// ExecPipeline encodes N RESP commands on one borrowed connection, flushes once, and reads N ordered slots. Empty or nil commands returns nil, nil and does not dial. More than 64 commands returns redis:issue? and does not send. The batch error is only I/O, protocol, cap, unreachable, or timeout; a per-element - reply lives on that slot's Err.
func (sr *SimpleRedis) ExecPipeline(commands [][][]byte) ([]PipelineSlot, error) {
	if len(commands) == 0 {
		return nil, nil
	}
	if len(commands) > maxPipelineCommands {
		return nil, errIssue
	}
	return sr.execPipeline(commands)
}

// execPipeline borrows a connection, writes N frames, flushes once, and retries retryable failures only before Flush.
func (sr *SimpleRedis) execPipeline(commands [][][]byte) ([]PipelineSlot, error) {
	maxRetries, minBackoff, maxBackoff := retryLimits(sr.maxRetries, sr.minRetryBackoff, sr.maxRetryBackoff)
	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBackoff(attempt, minBackoff, maxBackoff))
		}
		conn, err := sr.borrow()
		if err != nil {
			if sr.isClosed() || !shouldRetry(err) {
				return nil, err
			}
			last = err
			continue
		}
		slots, reusable, flushed, err := sr.doPipeline(conn, commands)
		sr.release(conn, reusable)
		if err == nil {
			return slots, nil
		}
		// After Flush, do not send the batch again (double-apply).
		if flushed || !shouldRetry(err) {
			return slots, err
		}
		last = err
	}
	return nil, last
}
