package s3

import (
	"io"
	"testing"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cactus/go-statsd-client/statsd"
	"github.com/pkg/errors"
)

type predictableDownloader struct {
	err error
}

func (d *predictableDownloader) Download(w io.WriterAt, i *s3.GetObjectInput, o ...func(*s3manager.Downloader)) (int64, error) {
	if d.err != nil {
		return 0, d.err
	}
	return 1, nil
}

var checkTests = []struct {
	name string
	err  error
}{
	{"underwhelming", nil},
	{"kaboom", errors.New("boom!")},
}

func TestS3Check(t *testing.T) {
	l, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("zap.NewDevelopment(): %v", err)
	}
	for _, tt := range checkTests {
		// NewNoopClient never returns an error.
		s, _ := statsd.NewNoopClient()
		d := &predictableDownloader{err: tt.err}

		check, err := New(tt.name, s, Downloader(d), Logger(l))
		if err != nil {
			t.Errorf("New(%v, %v, %v, %v, Downloader(%v)): %v", tt.name, s, d, err)
			continue
		}

		if err := check.Check(); err != nil {
			if tt.err == nil {
				t.Errorf("Got %v, did not want error.", err)
				continue
			}
			continue
		}
		if tt.err != nil {
			t.Errorf("Got no error, wanted check to fail due to downloader error: %v", tt.err)
		}
	}
}
