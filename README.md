# kubernary
A canary app for Kubernetes (or anywhere really).

## Usage
Kubernary runs as a daemon. It runs all checks at a configurable interval.
Checks generally emit logs and metrics at run time.

```
$ docker run negz/kubernary /kubernary/kubernary --help
usage: kubernary [<flags>] <statsd>

Checks whether your Kubernetes cluster works.

Flags:
      --help             Show context-sensitive help (also try --help-long and
                         --help-man).
      --no-stats         Don't send statsd stats.
      --listen=":10002"  Address at which to expose HTTP health checks.
  -d, --debug            Run with debug logging.
      --close-after=1m   Wait this long at shutdown before closing HTTP
                         connections.
      --kill-after=2m    Wait this long at shutdown before exiting.

Args:
  <statsd>  Address to which to send statsd metrics.
```

Kubernary exposes the following HTTP endpoints:

* `http://kubernary/quitquitquit` - Causes Kubernary to shutdown and exit
   immediately.
* `http://kubernary/health` - Runs all checks on-demand.

The `/health` endpoint returns:

* `200 OK` - If all checks pass.
* `503 SERVICE UNAVAILABLE` - If one or more check fails.
* `500 INTERNAL SERVER ERROR` - If an error unrelated to a check occurs.

Along with the following JSON body:
```
{
  "s3": {
    "ok": true,
    "error": ""
  },
  "failingcheck": {
    "ok": false,
    "error": "Kaboom!"
  }
}
```

## Checks
Currently the only check is Amazon S3. This check ensures a Kubernary can read a
file from an S3 bucket, primarily as a way of validating that `kube2iam` is
functioning correctly in a Kubernetes cluster.

The check uses the following environment variables for configuration:
* `KUBERNARY_S3_BUCKET` - The bucket to read from.
* `KUBERNARY_S3_KEY` - The key to read within the bucket. Reading a very small
  or zero length file is recommended.

The following statsd metrics are emitted by the check:
* `kubernary.s3.download.succeeded` - A count of successful S3 downloads.
* `kubernary.s3.download.failed` - A count of failed S3 downloads.

## Building
To build a Docker image run the following with a working Go environment:
```
$ scripts/build.sh
```

This (possibly somewhat brittle) script will build a Kubernary binary, and copy
it into a Docker image. You'll need to push it to ECR manually:
```
VERSION=$(git rev-parse -short HEAD)

docker push negz/goodneighbor:latest
docker push negz/goodneighbor:${VERSION}
```