# Hylo server with the Pi agent. Pi setup follows Pi's docs/containerization.md.
# Persistent state lives in /data (HOME): server config and key, vault, Pi credentials and sessions.

FROM golang:1.26-alpine AS build
RUN apk add --no-cache g++ musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev COMMIT=unknown BUILD_DATE=unknown
ARG MODULE=github.com/hardhacker/hylo
RUN CGO_ENABLED=1 go build -trimpath -buildvcs=false \
      -ldflags "-s -w -linkmode external -extldflags -static \
        -X $MODULE/internal/build.Version=$VERSION \
        -X $MODULE/internal/build.Commit=$COMMIT \
        -X $MODULE/internal/build.BuildDate=$BUILD_DATE" \
      -o /out/hylo ./cmd/hylo

FROM node:24-bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       bash ca-certificates git openssh-client ripgrep fd-find tini \
  && ln -s /usr/bin/fdfind /usr/local/bin/fd \
  && rm -rf /var/lib/apt/lists/*

ARG PI_VERSION=1.4.0
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent@${PI_VERSION} \
  && npm cache clean --force

# node:24 ships uid 1000 as "node"; reuse it rather than fight over the id.
RUN usermod -l hylo -d /data -m node && groupmod -n hylo node \
  && mkdir -p /data && chown hylo:hylo /data

COPY --from=build /out/hylo /usr/local/bin/hylo
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER hylo
ENV HOME=/data SHELL=/bin/bash PI_SKIP_VERSION_CHECK=1 PI_TELEMETRY=0
WORKDIR /data
VOLUME /data
EXPOSE 54321

# tini reaps the agent processes hylo spawns, since hylo runs as PID 1.
ENTRYPOINT ["tini", "--", "docker-entrypoint.sh"]
CMD ["hylo", "start", "server", "--pid-file", ""]
