# OSS Scanner 的构建环境：联网阶段装好全部依赖并完成构建，之后的分析在断网环境中进行
FROM node:26-bookworm-slim AS node

FROM golang:1.27-bookworm
# GOTOOLCHAIN=auto 让 go.mod 要求的工具链高于镜像自带版本时在构建阶段自动下载
ENV DEBIAN_FRONTEND=noninteractive \
    GOTOOLCHAIN=auto \
    GOFLAGS=-buildvcs=false \
    CGO_ENABLED=0
# 压缩工具是面板文件管理的运行时依赖，pkg/io 的测试也要用
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git iproute2 jq openssl procps sqlite3 \
      bzip2 p7zip-full unzip xz-utils zip zstd \
    && rm -rf /var/lib/apt/lists/*

# Node 版本与 .github/workflows/build.yml 保持一致
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm

COPY . /src
WORKDIR /src

# pnpm 版本取自 web/package.json 的 packageManager
RUN npm install -g "$(node -p "require('./web/package.json').packageManager")"

# 前端产物由 pnpm build 复制到 pkg/embed/frontend，后端编译时嵌入
RUN cd web \
    && cp .env.production .env \
    && cp config/proxy-config.example.ts config/proxy-config.ts \
    && pnpm install --frozen-lockfile \
    && pnpm run gettext:compile \
    && pnpm build

# 依赖全部进模块缓存，测试二进制预编译一遍，断网后仍能重新编译和跑测试
RUN go mod download \
    && go build -o /opt/ace/panel/ace ./cmd/ace \
    && go build -o /opt/ace/panel/cli ./cmd/cli \
    && go test -count=1 -run '^$' ./...

# 测试失败不中断构建
RUN go test -count=1 ./... || echo "WARNING: go test reported failures"

# 按真实安装的目录布局放好配置，启动方式见 threat_model.md
RUN mkdir -p /opt/ace/panel/storage \
    && sed -e 's/^  tls: .*/  tls: "off"/' -e '/^  acme: /d' config.example.yml > /opt/ace/panel/storage/config.yml \
    && chmod 600 /opt/ace/panel/storage/config.yml
