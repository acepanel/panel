# AGENTS.md

本文件为在此仓库工作的编码代理提供指引，`CLAUDE.md` 是指向它的软链接。

AcePanel 是 Go 写的 Linux 服务器运维面板，前后端分离。后端：Go + go-chi + GORM（SQLite）+ Wire。前端在 `web/`：Vue 3 + Vite + Naive UI + UnoCSS + Alova，构建产物经 `pkg/embed` 嵌进后端二进制。

## 协作约定

- 注释、提交说明、回复一律用简体中文。
- 只写代码，不创建 README、GUIDE、设计说明这类文档文件。
- 回复只说结论和必要信息，没被问到的不展开；改完直接结束，功能由维护者手动测试。
- 消除边界情况优于增加条件判断；只解决实际存在的问题，不为假想的场景加代码。
- 发现语义缺陷直接修，不为向后兼容保留错误行为。
- 不必处理命令注入、SQL 注入、文件上传这类问题：面板的登录用户都是服务器管理员。
- 不熟悉的库或功能，改代码前先查官方文档（Context7）或 GitHub 上的真实用例。

## 命令

后端，在仓库根目录执行：

- 构建：`go build -o ace ./cmd/ace`（面板）、`go build -o cli ./cmd/cli`（命令行工具）
- 测试：`go test ./...`，单个测试 `go test ./internal/biz/ -run TestCertRenewFromPath`
- Lint：`GOOS=linux golangci-lint run ./...`
- 依赖注入：改了构造函数或 `wire.go` 后执行 `go generate ./cmd/...`，校验用 `go tool wire check ./...`
- Mock：改了 `internal/biz` 里的接口后执行 `go run github.com/vektra/mockery/v3@v3.9.0`

面板只运行在 Linux，不少文件带 `//go:build linux`。在其它系统上构建和 lint 要加 `GOOS=linux`，否则这些文件不会被编译和检查。

前端，在 `web/` 目录执行，包管理器是 pnpm：

- `pnpm install`，`pnpm dev`（带热重载）
- 检查：`pnpm type-check`、`pnpm lint`（oxlint 加 eslint，会自动修复）、`pnpm format`
- `pnpm build`：类型检查、打包，并把 `dist` 复制到 `pkg/embed/frontend`
- 前端没有单元测试

首次开发前准备配置：

```bash
cp config.example.yml config.yml
cd web && cp .env.production .env && cp config/proxy-config.example.ts config/proxy-config.ts
```

CI 会跑 golangci-lint、`go tool wire check`、带 `-race` 的全量测试、mock 是否最新、`pnpm lint`，提交前相关检查应当能过。

## 后端架构

依赖方向是 `route → service → biz ← data`：biz 定义接口和领域模型，data 实现接口。

- `internal/route`：路由表。每个端点是一条 `Endpoint`，除 Method、Path、Handler 外要写 Summary、Tags 和 `Describe[请求, 响应]()`，API 文档由此生成；默认要求登录，公开端点设 `Public: true`。
- `internal/service`：解析请求、调用 biz、组织响应，不放业务逻辑。统一用 `Bind[T](r)`、`Success`、`Error`、`ErrorSystem`、`Paginate`。
- `internal/request`：请求结构体和校验标签；自定义校验规则在 `internal/rule`。
- `internal/biz`：`XxxRepo` 接口、`XxxUsecase` 和模型。`internal/data`：Repo 的实现，数据库、文件和系统命令都在这一层。
- `internal/apps/<slug>`：子应用（Nginx、MySQL、Redis 等），实现 `types.App` 的 `Route` 和 `Status`，挂在 `/api/apps/<slug>`，公共逻辑在 `internal/apps/common`。
- `internal/command`：CLI 命令定义，逻辑在 service 的 `CliService`。
- `internal/job` 定时任务，`internal/taskqueue` 后台任务队列，`internal/migration` 数据库迁移，`internal/bootstrap` 启动装配。
- `pkg/webserver`：Web 服务器抽象，nginx、apache、caddy、openlitespeed 各是一个方言。网站和证书逻辑通过 `webserver.Get(类型)` 取方言，不直接拼某一种服务器的配置。
- `pkg/` 下其余是与面板业务无关的独立工具包。

新增功能按层落位：route 加端点、service 加方法、biz 加接口和用例、data 加实现，再到各层的 `wire.go` 里 `Provide` 新的构造函数。动手前先读同层一个相近的现有实现。

容易踩的坑：

- 模型加字段后，要在 `internal/migration/v1.go` 末尾追加一条迁移（ID 形如 `20261116-add-cert-path`，内容是 `AutoMigrate`），否则已安装的面板不会加列。
- 校验器开了严格 required，`required` 会拒绝零值。规则之间用 `&&` 连接，跨字段规则引用 Go 字段名，如 `required_if:Auth,true`、`required_with:CertPath`。新字段的显示名登记在 `internal/request/attributes.go`。
- `internal/mocks` 里的 mock 带编译期接口断言，biz 接口一改不重新生成就编译不过。
- 测试断言用 `github.com/libtnb/assert` 的 `check` 和 `must`。mock 上没设桩的方法被调用会 panic，可以借此断言“不应调用”。

## 后端约定

- 对外返回或写进日志的文案都用 `t.Get("english text")`，msgid 必须是字符串字面量，翻译提取靠它。
- 操作日志：变更类操作成功后在 biz 层记一条，读操作（列表、详情、配置、日志查看）在 service 层记。写法：

  ```go
  uc.log.Info(uc.t.Get("cert uploaded"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)))
  ```

  删除、清空、卸载这类用 `Warn`；后台任务的 `operator_id` 写 0；属性名用 snake_case，不记密码、密钥、令牌。
- 只为某个类型服务的辅助函数写成它的未导出方法，不堆包级函数；新声明放到所属位置，不追加在文件末尾。
- 注释只写非显然的原因，一行以内；`//nolint:xxx` 后面不带理由。
- 集合处理用 `github.com/samber/lo`。
- 某种 Web 服务器不支持的配置项：前端不显示，后端不写入。

## 前端约定

- `ref`、`computed`、`useRequest`、VueUse 的函数都已自动导入，`src/components` 下的组件自动注册；`views/` 下的组件要显式 import。
- 页面路由写在各模块的 `views/<模块>/route.ts`，自动收集；子应用页面是 `views/apps/<slug>/IndexView.vue`；接口定义在 `src/api/panel/<模块>/index.ts`。
- 请求用 Alova 的 `useRequest` 等助手，不写 `onError`，错误已全局处理。
- 用户可见的文案都用 `$gettext('English text')` 包裹，源文案写英文。同一句英文在不同场景含义不同时，用 `$pgettext('context', 'English text')` 区分。
- 弹窗用 `<app-modal size="sm|md|lg|full">`，不直接写 `n-modal`，`preset="dialog"` 除外。
- 响应式：手机断点 768px，用 `useResponsive()` 的 `isMobile` 判断；表单布局用 `useFormLayout`，表格分页用 `usePagedTable` 或 `useTablePagination`；栅格用 `n-grid` 加 `responsive="screen"`。
- 表格列宽按中英文里更长的那份文案估算，避免表头换行。
- 图标用 `<i-mdi-xxx />`，工具类样式用 UnoCSS。样式问题在组件或主题里解决，不用全局 `!important` 覆盖。

## 翻译

译文和 `.pot` 文件（`web/src/locales/`、`pkg/embed/locales/`）由 Crowdin 和手动触发的 L10n 工作流（`.github/workflows/l10n.yml`）维护，不要手动编辑。新文案在译文同步之前显示英文原文，属于正常现象。

## 提交

提交说明写成 `type(scope): 中文概述`，正文分点写用户能感知到的变化。
