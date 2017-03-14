# kubernary
A canary app for Kubernetes (or anywhere really).

## Checks
Currently the only check is Amazon S3. This check ensures a Kubernary can read a
file from an S3 bucket, primarily as a way of validating that `kube2iam` is
functioning correctly in a Kubernetes cluster.

The check uses the following environment variables for configuration:
* `KUBERNARY_S3_BUCKET` - The bucket to read from.
* `KUBERNARY_S3_KEY` - The key to read within the bucket. Reading a very small
  or zero length file is recommended.

The following statsd metrics are emitted by the check:
* `kubernary.s3.download` - A boolean gauge. 1 when S3 downloads are successful.

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