package kubernary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// CheckConfigEnvPrefix is the prefix required by any check configuration
// environment variables.
const CheckConfigEnvPrefix string = "KUBERNARY_"

// A Checker is a health check run by kubernary to assess cluster functionality.
type Checker interface {
	Check() error
	Name() string
}

// A CheckConfig specified how a check should be run.
type CheckConfig struct {
	Checker  Checker
	Interval time.Duration
	Timeout  time.Duration
}

// RunCheckForever causes a check to be run every configured interval, forever.
func RunCheckForever(cfg *CheckConfig) context.CancelFunc {
	t := time.NewTicker(cfg.Interval)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-t.C:
				// Disable linters concerned with unchecked errors.
				// There is nothing for us to do with an error in this context.
				// Emitting logs and metrics for failed checks is the
				// responsibility of the checker.
				go func() { cfg.Checker.Check() }() // nolint: gas,errcheck
			case <-ctx.Done():
				t.Stop()
				return
			}
		}
	}()
	return cancel
}

// RunChecksForever causes a slice of checks to be run every configured
// interval, forever.
func RunChecksForever(cfgs []*CheckConfig) context.CancelFunc {
	cancels := make([]context.CancelFunc, 0, len(cfgs))
	for _, cfg := range cfgs {
		cancels = append(cancels, RunCheckForever(cfg))
	}
	return func() {
		for _, cancel := range cancels {
			cancel()
		}
	}
}

type e struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func longestTimeoutOf(cfgs []*CheckConfig) time.Duration {
	timeout := 0 * time.Nanosecond
	for _, cfg := range cfgs {
		if cfg.Timeout > timeout {
			timeout = cfg.Timeout
		}
	}
	return timeout
}

func runChecks(ctx context.Context, cfgs []*CheckConfig) map[string]error {
	ctx, cancel := context.WithTimeout(ctx, longestTimeoutOf(cfgs))
	defer cancel()

	type result struct {
		name string
		err  error
	}
	wg := &sync.WaitGroup{}
	rs := make(chan *result, len(cfgs))
	for _, cfg := range cfgs {
		wg.Add(1)
		go func(cfg *CheckConfig) { rs <- &result{cfg.Checker.Name(), cfg.Checker.Check()} }(cfg)
	}
	allChecksDone := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		close(allChecksDone)
	}()
	results := map[string]error{}
	for {
		select {
		case result := <-rs:
			results[result.name] = result.err
			wg.Done()
		case <-ctx.Done():
			for _, cfg := range cfgs {
				if _, ok := results[cfg.Checker.Name()]; !ok {
					results[cfg.Checker.Name()] = errors.Wrap(ctx.Err(), "check timed out")
				}
			}
			return results
		case <-allChecksDone:
			return results
		}
	}
}

func sendJSONCheckResults(w http.ResponseWriter, errs map[string]error) error {
	results := map[string]*e{}
	for name, err := range errs {
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			results[name] = &e{Error: err.Error()}
			continue
		}
		results[name] = &e{OK: true}
	}
	j, err := json.Marshal(results)
	if err != nil {
		return errors.Wrap(err, "cannot marshal check statuses")
	}
	_, err = w.Write(j)
	return errors.Wrap(err, "cannot write healthy check status")
}

// ChecksHandler returns an HTTP handler that runs the provided checks
// concurrently and returns the results.
func ChecksHandler(cfgs []*CheckConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := sendJSONCheckResults(w, runChecks(r.Context(), cfgs)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func sendJSONCheckResult(w http.ResponseWriter, err error) error {
	if err != nil {
		j, jerr := json.Marshal(&e{Error: err.Error()})
		if jerr != nil {
			return errors.Wrap(jerr, "cannot marshal unhealthy check status")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, werr := w.Write(j)
		return errors.Wrap(werr, "cannot write unhealthy check status")
	}
	j, jerr := json.Marshal(&e{OK: true})
	if jerr != nil {
		return errors.Wrap(jerr, "cannot marshal healthy check status")
	}
	_, err = w.Write(j)
	return errors.Wrap(err, "cannot write healthy check status")
}

// CheckHandler returns an HTTP handler that runs the provided check and returns
// the results.
func CheckHandler(cfg *CheckConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		cfgs := []*CheckConfig{cfg}
		results := runChecks(r.Context(), cfgs)
		for _, result := range results {
			if err := sendJSONCheckResult(w, result); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
}

// ShutdownHandler shuts down kubernary when called.
func ShutdownHandler(cancel context.CancelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		os.Exit(0)
	}
}

// CheckConfigFromEnv provides a standard pattern for reading basic check
// specific config from environment variables. Pass in a map of keys with
// default values. If KUBERNARY_CHECKNAME_KEY is set for key its default value
// will be overwritten.
func CheckConfigFromEnv(name string, config map[string]string) map[string]string {
	// TODO(negz): A better pattern for configuring individual checks.
	populated := map[string]string{}
	for k, v := range config {
		e, ok := os.LookupEnv(strings.ToUpper(fmt.Sprintf("%s%s_%s", CheckConfigEnvPrefix, name, k)))
		if !ok {
			e = v
		}
		populated[k] = e
	}
	return populated
}
