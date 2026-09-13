package simpleredis

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestConcurrentMSetEXUnknownCommandFallback locks that overlapping MSetEX
// unknown-command fallbacks stay successful and do not leak turns.
// Stays on under -race: the detector is the proof the groupWrite cache interleaving is safe.
func TestConcurrentMSetEXUnknownCommandFallback(t *testing.T) {
	workers := 16
	callsPerWorker := 25
	if raceDetectorOn {
		workers = 4
		callsPerWorker = 8
	}
	fake, addr := startFakeRedis(t, map[string]string{})
	fake.setRejectMSetEX()
	client := New(Config{Host: addr, PoolSize: 8})
	t.Cleanup(client.Close)

	var wg sync.WaitGroup
	errCh := make(chan error, workers*callsPerWorker)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(worker int) {
			defer wg.Done()
			for n := 0; n < callsPerWorker; n++ {
				name := fmt.Sprintf("w%d-%d", worker, n)
				if err := client.MSetEX(context.Background(), []string{name}, [][]byte{[]byte("v")}, 60); err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("MSetEX: %v", err)
	}
	assertTurnsFullAndNoOverFrees(t, client)
}
