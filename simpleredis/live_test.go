package simpleredis

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const timeWaitHoldScript = `local start = redis.call("TIME")
local startSec = tonumber(start[1])
local startUsec = tonumber(start[2])
local need = tonumber(ARGV[1])
while true do
  local now = redis.call("TIME")
  local elapsed = (tonumber(now[1]) - startSec) * 1000000 + (tonumber(now[2]) - startUsec)
  if elapsed >= need then
    break
  end
end
return 1`

func TestLive_RedisAndDragonfly(t *testing.T) {
	if testing.Short() {
		t.Skip("live engines skipped under -short")
	}
	backends := []struct {
		name string
		addr string
	}{
		{"redis", os.Getenv("SIMPLEREDIS_LIVE_REDIS")},
		{"dragonfly", os.Getenv("SIMPLEREDIS_LIVE_DRAGONFLY")},
	}
	anyAddr := false
	for _, backend := range backends {
		if backend.addr == "" {
			continue
		}
		anyAddr = true
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			runLivePoolBackend(t, backend.addr)
		})
	}
	if !anyAddr {
		t.Skip("SIMPLEREDIS_LIVE_REDIS and SIMPLEREDIS_LIVE_DRAGONFLY unset")
	}
}

func runLivePoolBackend(t *testing.T, addr string) {
	t.Helper()
	client := waitLiveSimpleRedis(t, addr)
	t.Cleanup(client.Close)

	t.Run("overlappingHoldsStayWithinEight", func(t *testing.T) {
		requireProcNet(t)
		const holders = 8
		const holdUs = "500000"
		started := make(chan struct{}, holders)
		var wg sync.WaitGroup
		errs := make(chan error, holders)
		for i := 0; i < holders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				started <- struct{}{}
				_, err := client.Eval(timeWaitHoldScript, nil, []string{holdUs})
				errs <- err
			}()
		}
		for i := 0; i < holders; i++ {
			<-started
		}
		port := livePort(t, addr)
		deadline := time.Now().Add(250 * time.Millisecond)
		for countEstablishedToPort(port) < holders {
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		live := countEstablishedToPort(port)
		if live < 1 {
			t.Fatal("no ESTABLISHED sockets to the live engine")
		}
		if live > holders+2 {
			t.Fatalf("live clients %d, want at most %d plus healthchecks", live, holders)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil && err.Error() != RedisTimeout && err.Error() != RedisUnreachable {
				t.Fatalf("hold Eval: %v", err)
			}
		}
	})

	t.Run("ninthWaiterIsUnreachable", func(t *testing.T) {
		requireProcNet(t)
		const holders = 8
		const holdUs = "500000"
		started := make(chan struct{}, holders)
		var wg sync.WaitGroup
		for i := 0; i < holders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				started <- struct{}{}
				_, _ = client.Eval(timeWaitHoldScript, nil, []string{holdUs})
			}()
		}
		for i := 0; i < holders; i++ {
			<-started
		}
		port := livePort(t, addr)
		deadline := time.Now().Add(250 * time.Millisecond)
		for countEstablishedToPort(port) < holders {
			if time.Now().After(deadline) {
				t.Fatal("holders did not occupy the pool")
			}
			time.Sleep(10 * time.Millisecond)
		}
		_, err := client.Eval(timeWaitHoldScript, nil, []string{"1000"})
		wg.Wait()
		if err == nil || err.Error() != RedisUnreachable {
			t.Fatalf("ninth Eval = %v, want %s", err, RedisUnreachable)
		}
	})
}

func waitLiveSimpleRedis(t *testing.T, addr string) *SimpleRedis {
	t.Helper()
	client := &SimpleRedis{}
	client.Init(addr, "", "")
	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := client.Set("simpleredis-live-probe", []byte("1"), 60); err == nil {
			return client
		} else if time.Now().After(deadline) {
			t.Fatalf("live %s: %v", addr, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func requireProcNet(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		t.Skip("/proc/net/tcp required to count sockets while Lua holds the engine")
	}
}

func livePort(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("live addr %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("live port %q", portStr)
	}
	return port
}

func countEstablishedToPort(port int) int {
	hexPort := fmt.Sprintf("%04X", port)
	n := 0
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 4 || fields[3] != "01" {
				continue
			}
			remote := strings.ToUpper(fields[2])
			if strings.HasSuffix(remote, ":"+hexPort) {
				n++
			}
		}
	}
	return n
}
