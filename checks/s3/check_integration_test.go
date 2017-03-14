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
	envEndpoint   string = "S3_ENDPOINT"
	envRegion     string = "S3_REGION"
	defaultRegion string = "us-east-1"
)

func TestS3CheckIntegration(t *testing.T) {
	e := os.Getenv(envEndpoint)

	// NewNoopClient never returns an error.
	s, _ := statsd.NewNoopClient()

	region, ok := os.LookupEnv(envRegion)
	if !ok {
		region = defaultRegion
	}
	// Path style is necessary for https://github.com/jubos/fake-s3
	cfg := aws.NewConfig().WithEndpoint(e).WithRegion(region).WithS3ForcePathStyle(true)
	session, err := session.NewSession(cfg)
	if err != nil {
		t.Fatalf("Unable to create new AWS session against endpoint %v: %v", e, err)
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
			t.Fatalf("Want data at endpoint %s to exist, but check says it does not.", e)
		}
	})

	t.Run("KeyProbablyDoesNotExist", func(t *testing.T) {
		os.Setenv(fmt.Sprintf("%s_%s", kubernary.CheckConfigEnvPrefix, "KEYPROBABLYDOESNOTEXIST_BUCKET"), probablyDoesNotExist())
		os.Setenv(fmt.Sprintf("%s_%s", kubernary.CheckConfigEnvPrefix, "KEYPROBABLYDOESNOTEXIST_KEY"), probablyDoesNotExist())
		check, err := New("KeyProbablyDoesNotExist", s, Downloader(d), Logger(l))
		if err != nil {
			t.Fatalf("New(KeyProbablyDoesNotExist, %s, Downloader(%s), Logger(%s): %v", s, d, l)
		}
		if err := check.Check(); err == nil {
			t.Fatalf("Want data at endpoint %s to be absent, but check says it exists", e)
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
