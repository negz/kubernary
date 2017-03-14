#/bin/bash -e

S3_BUCKET_NAME=${1}
S3_DATA_DIR=${2}

[ -z "${DOCKER_HOST}" ] && echo "DOCKER_HOST must be set" && exit 1
[ -z "${S3_DATA_DIR}" ] && echo "usage: $0 <bucket> <directory to copy to bucket>" && exit 1

S3_PORT=10004
S3_HOST=$(echo ${DOCKER_HOST##*/}|sed 's/:.*//')  # proto://127.0.0.1:8080 -> 127.0.0.1
S3_ENDPOINT="http://${S3_HOST}:${S3_PORT}"

# This container still uses a pre-1.0 MIT licensed version.z
docker run -d -p ${S3_PORT}:4569 "lphoward/fake-s3" &>/dev/null

aws --endpoint-url=${S3_ENDPOINT} s3 mb s3://${S3_BUCKET_NAME} --region ${S3_REGION:-us-east-1}
aws --endpoint-url=${S3_ENDPOINT} s3 cp ${S3_DATA_DIR} s3://${S3_BUCKET_NAME} --recursive

echo ${S3_ENDPOINT}