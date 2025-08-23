# AcePanel Linux Server Management Panel

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Bootstrap and Dependencies
- Install Go 1.24+ and Node.js with pnpm:
  - `go version` -- verify Go 1.24+
  - `npm install -g pnpm` -- install pnpm package manager
  - `cd /home/runner/work/panel/panel && cp config.example.yml config.yml` -- copy required config file
- Download dependencies:
  - `go mod download` -- takes ~25 seconds. NEVER CANCEL. Set timeout to 60+ minutes.
  - `cd web && pnpm install` -- takes ~27 seconds. NEVER CANCEL. Set timeout to 60+ minutes.

### Build Process
- Build backend applications:
  - `go build -o ace ./cmd/ace` -- takes ~14 seconds. NEVER CANCEL. Set timeout to 30+ minutes.
  - `go build -o cli ./cmd/cli` -- takes ~1 second. NEVER CANCEL. Set timeout to 30+ minutes.
- Build frontend application:
  - `cd web && cp .env.production .env && cp settings/proxy-config.example.ts settings/proxy-config.ts`
  - `cd web && pnpm run gettext:compile` -- compile translations, takes ~1 second
  - `cd web && pnpm build` -- takes ~27 seconds. NEVER CANCEL. Set timeout to 60+ minutes.

### Testing and Linting
- Run Go tests:
  - `go test -v ./pkg/...` -- takes ~31 seconds. NEVER CANCEL. Set timeout to 60+ minutes.
  - **Expected Results**: Tests like cert, queue, os will PASS. Tests like acme, api, ntp, tools will FAIL due to network connectivity requirements (DNS lookups, external APIs) - this is expected in sandboxed environments.
  - **Working Tests**: cert (SSL certificate generation), queue (task queuing), os (OS detection), rsacrypto (encryption)
  - **Network-Dependent Tests**: acme (Let's Encrypt), api (external panel API), ntp (time sync), tools (public IP detection)
- Run Go linting:
  - `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
  - `~/go/bin/golangci-lint run --timeout=30m ./...` -- takes ~13 seconds. NEVER CANCEL. Set timeout to 60+ minutes.
- Run frontend linting:
  - `cd web && pnpm lint` -- takes ~7 seconds. NEVER CANCEL. Set timeout to 30+ minutes.

## Validation
- Always build both ace and cli applications after making Go code changes
- Always run `~/go/bin/golangci-lint run --timeout=30m ./...` before committing Go changes
- Always run `cd web && pnpm lint` before committing frontend changes
- **Manual Validation Steps:**
  - Verify binaries are built: `ls -la ace cli` -- should show executable files ~54MB (ace) and ~33MB (cli)
  - Test binary format: `file ace cli` -- should show "ELF 64-bit LSB executable"
  - Confirm frontend assets: `ls web/dist/` -- should contain built assets and index.html
- **CRITICAL Environment Limitations:**
  - Applications require root privileges and will panic with "panel must run as root" in non-root environments
  - Database setup required: Applications need SQLite database initialization to run functionally
  - Network dependencies: Many tests fail in sandboxed environments due to DNS/API restrictions
  - **DO NOT attempt to run ace or cli without proper root setup** - focus on build and lint validation
- For development testing, focus on build success and lint compliance rather than runtime testing
- **Always remove built binaries before committing**: `rm -f ace cli` to avoid including them in git

## Understanding the Codebase

### Architecture
- Backend: Go with chi router (migrating to Fiber v3) + wire dependency injection + GORM
- Frontend: Vue 3 + TypeScript + Vite + Naive UI + pnpm
- Database: SQLite3 by default, supports MySQL/PostgreSQL
- Config: YAML-based configuration in config.yml

### Key Directories
- `cmd/ace/` -- Main panel web server application
- `cmd/cli/` -- Command line interface for panel management  
- `internal/biz/` -- Business logic interfaces and domain models (similar to DDD domain layer)
- `internal/data/` -- Data access layer implementing biz interfaces (similar to DDD repository layer)
- `internal/service/` -- Service layer handling DTO to domain object conversion (similar to DDD application layer)
- `internal/route/` -- HTTP route definitions
- `pkg/` -- Shared utility packages and tools
- `web/` -- Vue 3 frontend application
- `storage/` -- Data storage directory

### Development Workflow
When adding new features:
1. Add routes in `internal/route/http`
2. Add service methods in `internal/service` (review existing services for patterns)
3. Add business interfaces in `internal/biz` (review existing interfaces for patterns)  
4. Implement data layer in `internal/data` (review existing implementations for patterns)

### Testing Patterns
- Uses testify/suite for Go testing
- Example test structure:
```go
type MyTestSuite struct {
    suite.Suite
}

func TestMyTestSuite(t *testing.T) {
    suite.Run(t, &MyTestSuite{})
}

func (s *MyTestSuite) TestMyFunction() {
    // test implementation
}
```

## Common Tasks Reference

### Repository Root Files
```
.air.toml           -- Air hot reload config
.github/            -- GitHub workflows and configs
.gitignore          -- Git ignore patterns
.goreleaser.yaml    -- Release configuration
cmd/                -- Application entry points
config.example.yml  -- Example configuration
go.mod             -- Go module definition
internal/          -- Private application code
pkg/               -- Public utility packages
storage/           -- Data storage
web/               -- Frontend Vue application
```

### Configuration
- Copy `config.example.yml` to `config.yml` for basic setup
- Default HTTP port: 8888
- Default root directory: `/www`
- Default locale: `zh_CN` (Chinese Simplified)

### Build Artifacts Location
- Backend binaries: Built in repository root as `ace` and `cli`
- Frontend assets: Built to `web/dist/` and copied to `pkg/embed/frontend/`

## 开发指南 (Development Guidelines)

严格按照用户要求执行，使用 Go 1.24 和现代最佳实践：

- 使用 github.com/gofiber/fiber/v3 和 gorm.io/gorm 进行开发
- Fiber v3 handler 使用 `c fiber.Ctx` 而不是 `c *fiber.Ctx`
- 遵循项目的 DDD 分层架构：biz → data → service → route
- 使用标准库 slog 进行日志记录
- 编写完整、安全、高效的代码，不留待办事项
- 使用 testify/suite 模式编写测试

## 项目描述

本项目是基于 Go 语言的 Fiber 框架和 wire 依赖注入开发的 AcePanel Linux 服务器运维管理面板，目前正在进行 v3 版本重构。

v3 版本需要完成以下重构任务：
1. 使用 Fiber v3 替换目前的 go-chi 路由
2. 全新的项目模块，支持运行 Java/Go/Python 等项目
3. 网站模块重构，支持多 Web 服务器（Apache/OLS/Kangle）
4. 备份模块重构，需要支持 s3 和 ftp/sftp 备份途径
5. 计划任务模块重构，支持管理备份任务和自定义脚本任务等

## 项目结构

├── cmd/
│   ├── ace/ 面板主程序
│   └── cli/ 面板命令行工具
├── internal/
│   ├── app/ 应用入口
│   ├── apps/ 面板各子应用的实现
│   ├── biz/ 业务逻辑的接口和数据库模型定义，类似 DDD 的 domain 层，data 类似 DDD 的 repo，而业务接口在这里定义，使用依赖倒置的原则
│   ├── bootstrap/ 各个模块的启动引导
│   ├── data/ 业务数据访问，包含 cache、db 等封装，实现了 biz 的业务接口。我们可能会把 data 与 dao 混淆在一起，data 偏重业务的含义，它所要做的是将领域对象重新拿出来，我们去掉了 DDD 的 infra 层
│   ├── http/
│   │   ├── middleware/ 自定义路由中间件
│   │   ├── request/ 请求结构体
│   │   └── rule/ 自定义验证规则
│   ├── job/ 面板后台任务
│   ├── migration/ 数据库迁移定义
│   ├── queuejob/ 面板任务队列
│   ├── route/ 路由定义
│   └── service/ 实现了路由定义的服务层，类似 DDD 的 application 层，处理 DTO 到 biz 领域实体的转换(DTO -> DO)，同时协同各类 biz 交互，但是不应处理复杂逻辑
├── mocks/ 模拟数据，目前没有使用
├── pkg/ 工具函数及包
├── storage/ 数据存储
└── web/ 前端项目

## 开发新需求时的流程

1. 在 route/http 中添加新的路由和注入需要的服务
2. 在 service 中添加新的服务方法，先读取已存在的其他服务方法，以参考它们的实现方式
3. 在 biz 中添加新的业务逻辑需要的接口等，先读取已存在的其他接口，以参考它们的实现方式
4. 在 data 中实现 biz 的接口，先读取已存在的其他实现，以参考它们的实现方式
