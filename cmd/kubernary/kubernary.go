package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/negz/kubernary"
	"github.com/negz/kubernary/checks/s3"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/facebookgo/httpdown"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const statsPrefix string = "kubernary"

func setupS3Check(log *zap.Logger, s statsd.Statter) *kubernary.CheckConfig {
	check, err := s3.New("s3", s, s3.Logger(log))
	kingpin.FatalIfError(err, "cannot setup S3 check")
	return &kubernary.CheckConfig{Checker: check, Interval: 3 * time.Second, Timeout: 2 * time.Second}
}

// TODO(negz): Find a better pattern for including and configuring checks.
func setupChecks(log *zap.Logger, s statsd.Statter) []*kubernary.CheckConfig {
	return []*kubernary.CheckConfig{setupS3Check(log, s)}
}

func logReq(fn http.HandlerFunc, log *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("request",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
			zap.String("addr", r.RemoteAddr))
		fn(w, r)
	}
}

func main() {
	var (
		app    = kingpin.New(filepath.Base(os.Args[0]), "Checks whether your Kubernetes cluster works.").DefaultEnvars()
		stats  = app.Arg("statsd", "Address to which to send statsd metrics.").Required().String()
		nosend = app.Flag("no-stats", "Don't send statsd stats.").Bool()
		listen = app.Flag("listen", "Address at which to expose HTTP health checks.").Default(":10002").String()
		debug  = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		stop   = app.Flag("close-after", "Wait this long at shutdown before closing HTTP connections.").Default("1m").Duration()
		kill   = app.Flag("kill-after", "Wait this long at shutdown before exiting.").Default("2m").Duration()
	)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	var log *zap.Logger
	log, err := zap.NewProduction()
	if *debug {
		log, err = zap.NewDevelopment()
	}
	kingpin.FatalIfError(err, "cannot create logger")

	s, err := statsd.NewNoopClient(*stats, statsPrefix)
	if !*nosend {
		s, err = statsd.NewClient(*stats, statsPrefix)
	}
	kingpin.FatalIfError(err, "cannot create statsd client")

	cfgs := setupChecks(log, s)

	cancel := kubernary.RunChecksForever(cfgs)

	r := httprouter.New()
	r.HandlerFunc("GET", "/health", logReq(kubernary.ChecksHandler(cfgs), log))
	r.HandlerFunc("GET", "/quitquitquit", logReq(kubernary.ShutdownHandler(cancel), log))

	hd := &httpdown.HTTP{StopTimeout: *stop, KillTimeout: *kill}
	http := &http.Server{Addr: *listen, Handler: r}

	kingpin.FatalIfError(httpdown.ListenAndServe(http, hd), "HTTP server error")
}
