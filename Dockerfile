FROM golang:1.22 as builder

ARG VERSION
WORKDIR /go/src/github.com/paul-carlton/goutils
COPY . .
RUN make

ENV TAG=$TAG \
  GIT_SHA=$GIT_SHA \
  BUILD_DATE=$BUILD_DATE \
  SRC_REPO=$SRC_REPO

LABEL TAG=$TAG \
  GIT_SHA=$GIT_SHA \
  BUILD_DATE=$BUILD_DATE \
  SRC_REPO=$SRC_REPO
