package s3

import (
	"github.com/negz/kubernary"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/cactus/go-statsd-client/statsd"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	metricDownloadSucceeded string = "download.succeeded"
	metricDownloadFailed    string = "download.failed"

	cfgRegion string = "REGION"
	cfgBucket string = "BUCKET"
	cfgKey    string = "KEY"

	defaultRegion string = "us-east-1"
	defaultBucket string = "kubernary"
	defaultKey    string = "check"
)

type check struct {
	name       string
	stats      statsd.SubStatter
	log        *zap.Logger
	downloader s3manageriface.DownloaderAPI
	bucket     string
	key        string
}

func newDownloader(region string) (s3manageriface.DownloaderAPI, error) {
	// TODO(negz): Make region configurable.
	s, err := session.NewSession(aws.NewConfig().WithRegion(region))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new AWS session")
	}
	return s3manager.NewDownloader(s), nil
}

// An Option represents an S3 checker option.
type Option func(*check) error

// Downloader allows the use of a bespoke S3 Downloader.
func Downloader(d s3manageriface.DownloaderAPI) Option {
	return func(c *check) error {
		c.downloader = d
		return nil
	}
}

// Logger allows the use of a bespoke Zap logger.
func Logger(l *zap.Logger) Option {
	return func(c *check) error {
		c.log = l
		return nil
	}
}

// New returns a Checker that checks whether the supplied S3 file is accessible.
func New(name string, s statsd.Statter, co ...Option) (kubernary.Checker, error) {
	l, err := zap.NewProduction()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create default logger")
	}

	cfg := map[string]string{
		cfgRegion: defaultRegion,
		cfgBucket: defaultBucket,
		cfgKey:    defaultKey,
	}
	cfg = kubernary.CheckConfigFromEnv(name, cfg)

	c := &check{
		name:   name,
		stats:  s.NewSubStatter(name),
		log:    l,
		bucket: cfg[cfgBucket],
		key:    cfg[cfgKey],
	}

	for _, o := range co {
		if err := o(c); err != nil {
			return nil, errors.Wrap(err, "cannot apply S3 Checker option")
		}
	}

	c.log = c.log.With(zap.String("checkName", c.name), zap.String("bucket", c.bucket), zap.String("key", c.key))

	if c.downloader == nil {
		var err error
		c.downloader, err = newDownloader(cfg[cfgRegion])
		if err != nil {
			return nil, errors.Wrap(err, "cannot create S3 downloader")
		}
	}

	return c, nil
}

func (c *check) checkCanDownload() error {
	_, err := c.downloader.Download(&aws.WriteAtBuffer{}, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(c.key),
	})
	if err != nil {
		if serr := c.stats.Inc(metricDownloadFailed, 1, 1.0); serr != nil {
			c.log.Error("cannot emit metric", zap.String("metric", metricDownloadFailed), zap.Error(serr))
		}
		c.log.Error("download check failed", zap.Error(err))
		return errors.Wrapf(err, "%s download check failed, bucket=%s, key=%s", c.name, c.bucket, c.key)
	}
	if err := c.stats.Inc(metricDownloadSucceeded, 1, 1.0); err != nil {
		c.log.Error("cannot emit metric", zap.String("metric", metricDownloadSucceeded), zap.Error(err))
	}
	c.log.Debug("download check succeeded")
	return nil
}

func (c *check) Check() error {
	return errors.Wrapf(c.checkCanDownload(), "%s download check failed", c.name)
}

func (c *check) Name() string {
	return c.name
}
