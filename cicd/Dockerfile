#syntax=docker/dockerfile-upstream:1.6
FROM goreleaser as deps

ARG GO_APP
ARG APP_USER=root

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN mkdir -p /go/bin /go/src/dist

WORKDIR /go/src

RUN <<EOT
  # If not built, build for current platform
  if [ -z "$(ls -A dist)" ]; then
    if [ ${TARGETARCH} = "arm" ]; then VARIANT=$(echo ${TARGETVARIANT} | sed 's/^v//'); fi
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${VARIANT} TAGS=${TAGS} goreleaser build --single-target --snapshot -f ${GORELEASER_CONFIG}
  fi
EOT

RUN <<EOT
  set -e

  GORELEASER_DIST_DIR="/go/src/dist"
  # If --single-target, artifacts.json only exists in os_arch target directory
  if [ ! -f ${GORELEASER_DIST_DIR}/artifacts.json ]; then
    GORELEASER_DIST_DIR="${GORELEASER_DIST_DIR}/${TARGETOS}_${TARGETARCH}"
  fi

  if [ ${TARGETARCH} = "arm" ]; then VARIANT=$(echo ${TARGETVARIANT} | sed 's/^v//'); fi
  BIN_PATH=$(jq -r ".[] |select(.type   == \"Binary\" and \
                                .name   == \"${GO_APP}\" and \
                                .goos   == \"${TARGETOS}\" and \
                                .goarch == \"${TARGETARCH}\" and \
                                (.goarm == \"${VARIANT}\" or .goarm == null)) | .path" < ${GORELEASER_DIST_DIR}/artifacts.json)

  if [ -z ${BIN_PATH} ]; then
    exit 1
  fi

  cp ${BIN_PATH} /go/bin
EOT

FROM alpine:3.19.0

ARG GO_APP
ENV GO_APP ${GO_APP}

COPY --from=deps --chmod=0755 --chown=$APP_USR:$APP_USER /go/bin/${GO_APP} /usr/local/bin/${GO_APP}
COPY --from=assets --chmod=0755 entrypoint.sh /entrypoint.sh

RUN apk add --no-cache ca-certificates && ln -s /usr/local/bin/${GO_APP} /${GO_APP}

USER $APP_USER:$APP_USER

ENTRYPOINT ["/entrypoint.sh"]
