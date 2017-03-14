FROM golang:1.8
MAINTAINER Nic Cope <n+docker@rk0n.org>

ENV APP /kubernary

RUN mkdir -p "${APP}"
COPY "dist/kubernary" "${APP}"