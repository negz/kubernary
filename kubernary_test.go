package kubernary

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
)

const testWillTimeout string = "timeout"

type predictableChecker struct {
	name string
	m    sync.Mutex
	err  error
	r    int
	do   func()
}

func (c *predictableChecker) Check() error {
	if c.do != nil {
		c.do()
	}
	c.run()
	return c.err
}

func (c *predictableChecker) Name() string {
	return c.name
}

func (c *predictableChecker) run() {
	c.m.Lock()
	c.r++
	c.m.Unlock()
}

func (c *predictableChecker) runs() int {
	c.m.Lock()
	defer c.m.Unlock()
	return c.r
}

var checkerTests = []struct {
	cfgs              []*CheckConfig
	cancelImmediately bool
}{
	{
		cfgs: []*CheckConfig{
			&CheckConfig{
				Checker:  &predictableChecker{name: "pass"},
				Interval: 100 * time.Millisecond,
				Timeout:  100 * time.Millisecond,
			},
			&CheckConfig{
				Checker:  &predictableChecker{name: "passmore"},
				Interval: 100 * time.Millisecond,
				Timeout:  200 * time.Millisecond,
			},
		},
		cancelImmediately: false,
	},
	{
		cfgs: []*CheckConfig{
			&CheckConfig{
				// Sleep for longer than the longest timeout of this config check (i.e. 200ms).
				Checker:  &predictableChecker{name: testWillTimeout, do: func() { time.Sleep(300 * time.Millisecond) }},
				Interval: 100 * time.Millisecond,
				Timeout:  100 * time.Millisecond,
			},
			&CheckConfig{
				Checker:  &predictableChecker{name: "suchpass"},
				Interval: 100 * time.Millisecond,
				Timeout:  200 * time.Millisecond,
			},
		},
		// Cancel this immediately during the RunChecksForever test because the
		// check that times out will result in less passing checks than the
		// calculation expects.
		cancelImmediately: true,
	},
	{
		cfgs: []*CheckConfig{
			&CheckConfig{
				Checker:  &predictableChecker{name: "verypass"},
				Interval: 500 * time.Millisecond,
				Timeout:  2 * time.Second,
			},
			&CheckConfig{
				Checker:  &predictableChecker{name: "failfailfail", err: errors.New("Boom!")},
				Interval: 100 * time.Millisecond,
				Timeout:  2 * time.Second,
			},
		},
		cancelImmediately: false,
	},
}

func longestIntervalOf(cfgs []*CheckConfig) time.Duration {
	interval := 0 * time.Nanosecond
	for _, cfg := range cfgs {
		if cfg.Interval > interval {
			interval = cfg.Interval
		}
	}
	return interval
}

func TestChecker(t *testing.T) {
	t.Run("RunChecksForever", func(t *testing.T) {
		for _, tt := range checkerTests {
			cancel := RunChecksForever(tt.cfgs)

			var longestInterval time.Duration
			if !tt.cancelImmediately {
				longestInterval = longestIntervalOf(tt.cfgs)
				t.Logf("Sleeping for longest interval %s + 20ms", longestInterval)
				time.Sleep(longestInterval + 20*time.Millisecond)
			}
			cancel()

			for _, cfg := range tt.cfgs {
				expectedRuns := 0
				if longestInterval > 0*time.Nanosecond && cfg.Interval > 0*time.Nanosecond {
					// Expected runs should be longest interval / interval
					expectedRuns = int(longestInterval / cfg.Interval)
					t.Logf("Expected runs for %s: %v (%s / %s)", cfg.Checker.Name(), expectedRuns, longestInterval, cfg.Interval)
				}

				p, ok := cfg.Checker.(*predictableChecker)
				if !ok {
					t.Fatal("cfg.Checker: wanted predictableChecker")
				}
				if p.runs() != expectedRuns {
					t.Errorf("%s p.runs(): Want %d, got %d", cfg.Checker.Name(), expectedRuns, p.runs())
					continue
				}
			}

		}
	})

	t.Run("CheckHandlers", func(t *testing.T) {
		for _, tt := range checkerTests {
			checksWillFail := false
			for _, cfg := range tt.cfgs {
				p, ok := cfg.Checker.(*predictableChecker)
				if !ok {
					t.Fatal("cfg.Checker: wanted predictableChecker")
				}
				if p.err != nil {
					checksWillFail = true
					t.Logf("Config %s has failing checks.", cfg.Checker.Name())
				}
				if cfg.Checker.Name() == testWillTimeout {
					checksWillFail = true
				}
			}
			w := httptest.NewRecorder()
			ChecksHandler(tt.cfgs)(w, httptest.NewRequest("GET", "/", nil))
			expectedStatus := http.StatusOK
			if checksWillFail {
				expectedStatus = http.StatusServiceUnavailable
				t.Log("This check config set should fail.")
			}
			if w.Code != expectedStatus {
				t.Errorf("w.Code: want %v, got %v", expectedStatus, w.Code)
				continue
			}

			results := &map[string]*e{}
			if err := json.Unmarshal(w.Body.Bytes(), results); err != nil {
				t.Errorf("json.Unmarshal(%v, %s): %v", w.Body, results, err)
			}
		}
	})

	t.Run("CheckHandler", func(t *testing.T) {
		for _, tt := range checkerTests {
			cfg := tt.cfgs[0]
			p, ok := cfg.Checker.(*predictableChecker)
			if !ok {
				t.Fatal("cfg.Checker: wanted predictableChecker")
			}
			checksWillFail := false
			if p.err != nil {
				checksWillFail = true
				t.Logf("Config %s has failing checks.", cfg.Checker.Name())
			}
			if cfg.Checker.Name() == testWillTimeout {
				checksWillFail = true
			}
			w := httptest.NewRecorder()
			CheckHandler(tt.cfgs[0])(w, httptest.NewRequest("GET", "/", nil))
			expectedStatus := http.StatusOK
			if checksWillFail {
				expectedStatus = http.StatusServiceUnavailable
				t.Log("This check config set should fail.")
			}
			if w.Code != expectedStatus {
				t.Errorf("w.Code: want %v, got %v", expectedStatus, w.Code)
				continue
			}

			result := &e{}
			if err := json.Unmarshal(w.Body.Bytes(), result); err != nil {
				t.Errorf("json.Unmarshal(%v, %s): %v", w.Body, result, err)
			}
		}
	})
}
