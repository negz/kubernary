// +build integration

package s3

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cactus/go-statsd-client/statsd"
	"github.com/negz/kubernary"
)

const (
	cfgEndpoint string = "ENDPOINT"

	defaultEndpoint string = "http://localhost:10002"
)

func TestS3CheckIntegration(t *testing.T) {
	env := map[string]string{
		cfgRegion:   defaultRegion,
		cfgEndpoint: defaultEndpoint,
	}
	env = kubernary.CheckConfigFromEnv("s3_it", env)

	// NewNoopClient never returns an error.
	s, _ := statsd.NewNoopClient()

	// Path style is necessary for https://github.com/jubos/fake-s3
	cfg := aws.NewConfig().WithEndpoint(env[cfgEndpoint]).WithRegion(env[cfgRegion]).WithS3ForcePathStyle(true)
	session, err := session.NewSession(cfg)
	if err != nil {
		t.Fatalf("Unable to create new AWS session against endpoint %v: %v", env[cfgEndpoint], err)
	}
	d := s3manager.NewDownloader(session)

	l, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("zap.NewDevelopment(): %v", err)
	}

	t.Run("KeyExists", func(t *testing.T) {
		check, err := New("KeyExists", s, Downloader(d), Logger(l))
		if err != nil {
			t.Fatalf("New(KeyExists, %s, Downloader(%s), Logger(%s): %v", s, d, l)
		}
		if err := check.Check(); err != nil {
			t.Fatalf("Want data at endpoint %s to exist, but check says it does not.", env[cfgEndpoint])
		}
	})

	t.Run("KeyProbablyDoesNotExist", func(t *testing.T) {
		b := fmt.Sprintf("%s%s", kubernary.CheckConfigEnvPrefix, "KEYPROBABLYDOESNOTEXIST_BUCKET")
		k := fmt.Sprintf("%s%s", kubernary.CheckConfigEnvPrefix, "KEYPROBABLYDOESNOTEXIST_BUCKET")
		os.Setenv(b, probablyDoesNotExist())
		os.Setenv(k, probablyDoesNotExist())
		check, err := New("KeyProbablyDoesNotExist", s, Downloader(d), Logger(l))
		if err != nil {
			t.Fatalf("New(KeyProbablyDoesNotExist, %s, Downloader(%s), Logger(%s): %v", s, d, l)
		}
		if err := check.Check(); err == nil {
			t.Fatalf("Want data at endpoint %s to be absent, but check says it exists", env[cfgEndpoint])
		}
	})
}

func probablyDoesNotExist() string {
	rand.Seed(time.Now().Unix())
	l := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, rand.Intn(10-5)+5)
	for i := range b {
		b[i] = l[rand.Intn(len(l))]
	}
	return string(b)
}
