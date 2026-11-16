FROM node:26-bookworm-slim AS node

FROM golang:1.27-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    GOTOOLCHAIN=auto \
    GOFLAGS=-buildvcs=false \
    CGO_ENABLED=0

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git iproute2 jq openssl procps sqlite3 \
      bzip2 p7zip-full unzip xz-utils zip zstd \
    && rm -rf /var/lib/apt/lists/*

COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm

COPY . /src
WORKDIR /src

RUN npm install -g "$(node -p "require('./web/package.json').packageManager")"

RUN cd web \
    && cp .env.production .env \
    && cp config/proxy-config.example.ts config/proxy-config.ts \
    && pnpm install --frozen-lockfile \
    && pnpm run gettext:compile \
    && pnpm build

RUN go mod download \
    && go build -o /opt/ace/panel/ace ./cmd/ace \
    && go build -o /opt/ace/panel/cli ./cmd/cli \
    && go test -count=1 -run '^$' ./...

RUN go test -count=1 ./... || echo "WARNING: go test reported failures"

RUN mkdir -p /opt/ace/panel/storage \
    && sed -e 's/^  tls: .*/  tls: "off"/' -e '/^  acme: /d' config.example.yml > /opt/ace/panel/storage/config.yml \
    && chmod 600 /opt/ace/panel/storage/config.yml
