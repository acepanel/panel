## 项目概述

AcePanel 是基于 Go 语言开发的新一代 Linux 服务器运维管理面板。项目采用前后端分离架构：

- 后端：Go 1.26 + go-chi 路由 + GORM + Wire 依赖注入
- 前端：Vue 3 + Vite + Pinia + Naive UI + pnpm + xterm.js + Alova.js

## 核心原则

- **效率至上**：快速单元式开发，所有代码注释、文档和回复使用简体中文
- **不写文档**：只写代码，不创建 README、GUIDE 等各种文档
- **改完即退**：完成代码修改后立即退出，用户会手动测试
- **简洁执念**：消除边界情况永远优于增加条件判断，复杂性是万恶之源
- **实用主义**：解决实际问题，而不是假想的威胁
- **闭嘴**：非用户要求不输出任何内容，静默改完代码后直接退出

## 构建和测试

### 后端构建

构建主程序：

```bash
go build -o ace ./cmd/ace
```

构建 CLI 工具：

```bash
go build -o cli ./cmd/cli
```

### 前端构建和测试

进入前端目录：

```bash
cd web
```

安装依赖：

```bash
pnpm install
```

开发模式（带热重载）：

```bash
pnpm dev
```

构建生产版本：

```bash
pnpm build
```

## 代码架构

项目采用类 DDD 分层架构，依赖关系为：route -> service -> biz <- data

### 核心目录结构

- **`cmd/`**: 程序入口
    - `ace/`: 面板主程序
    - `cli/`: 命令行工具

- **`internal/app/`**: 应用入口和配置

- **`internal/route/`**: HTTP 路由定义
    - 定义路由规则
    - 注入所需的 service 依赖

- **`internal/service/`**: 服务层（类似 DDD 的 application 层）
    - 处理 HTTP 请求/响应
    - DTO 到 DO 的转换
    - 协调多个 biz 接口完成业务流程
    - **不应处理复杂业务逻辑**

- **`internal/biz/`**: 业务逻辑层（类似 DDD 的 domain 层）
    - 定义业务接口（Repository 模式）
    - 定义领域模型和数据结构
    - 使用依赖倒置原则：biz 定义接口，data 实现接口

- **`internal/data/`**: 数据访问层（类似 DDD 的 repository 层）
    - 实现 biz 中定义的业务接口
    - 封装数据库、缓存等操作
    - 处理数据持久化逻辑

- **`internal/middleware/`**: 自定义中间件

- **`internal/request/`**: 请求结构体定义

- **`internal/rule/`**: 自定义验证规则

- **`internal/apps/`**: 面板子应用实现

- **`internal/mocks/`**: mockery 生成的 biz 接口 mock，改接口后跑 `go run github.com/vektra/mockery/v3@v3.9.0` 重新生成

- **`internal/bootstrap/`**: 各模块启动引导

- **`internal/migration/`**: 数据库迁移

- **`internal/job/`**: 定时任务

- **`internal/taskqueue/`**: 任务队列运行器（基于 DB 轮询，实现 `types.TaskRunner` 接口）

- **`pkg/`**: 工具函数和通用包
    - 包含各种独立的工具模块
    - 可被项目任何部分引用

- **`web/`**: Vue 3 前端项目

## 开发新功能的标准流程

1. **在 `internal/route/` 中添加路由**
    - 参考已有路由文件（如 `http.go`）
    - 注入需要的 service 依赖
    - 定义路由规则和 handler 映射

2. **在 `internal/service/` 中实现服务方法**
    - **先阅读已有的类似服务**以了解代码风格
    - 处理请求验证和响应格式化
    - 使用 `Success()` 返回成功响应
    - 使用 `Error()` 返回错误响应
    - 使用 `ErrorSystem()` 返回系统严重错误
    - 调用 biz 层接口完成业务逻辑

3. **在 `internal/biz/` 中定义业务接口**
    - **先阅读已有的类似接口定义**
    - 定义 Repository 接口（如 `WebsiteRepo`）
    - 定义领域模型结构体（如 `Website`）
    - 保持接口简洁明确

4. **在 `internal/data/` 中实现 biz 接口**
    - **先阅读已有的类似实现**
    - 创建 repo 结构体（如 `websiteRepo`）
    - 实现构造函数（如 `NewWebsiteRepo`）
    - 实现所有接口方法
    - 处理数据库操作和缓存逻辑

5. **使用 Wire 进行依赖注入**
    - 在对应的 wire.go 文件中添加 provider
    - 运行 `go generate` 生成依赖注入代码

## 技术栈特定注意事项

### 助手函数（service 层）

在 service 层使用以下助手函数：

- `Success(w, data)`: 返回成功响应
- `Error(w, statusCode, format, args...)`: 返回错误响应
- `ErrorSystem(w, format, args...)`: 返回系统严重错误（500）
- `Bind[T](r)`: 绑定请求参数到泛型类型 T
- `Paginate[T](...)`: 构建分页响应

## 代码风格

- 复杂逻辑添加注释说明，简单逻辑不添加注释
- 使用 `github.com/samber/lo` 进行函数式编程辅助
- 后端对外返回的字符串需要尽可能使用 gotext 进行翻译处理
- 前端使用 gettext 进行国际化处理，所有用户可见的字符串都必须使用 gettext 包裹以支持翻译
- 禁止手动编辑前后端的翻译文件，项目由外部 Crowdin 自动化管理
- 前端 HTTP 请求使用 Alova.js 的 useRequest 等助手函数，无需添加 onError 错误处理
- 后端使用 Wire 依赖注入，添加新依赖时需要 `go generate ./...` 重新生成代码
- 无需关注命令注入，SQL注入，文件上传等安全问题（这是服务器面板，所有登录的用户都被视为管理员）


## 配置文件

后端开发配置：

```bash
cp config.example.yml config.yml
```

前端开发配置：

```bash
cd web
cp .env.production .env
cp settings/proxy-config.example.ts settings/proxy-config.ts
```

## 工具使用

对于不熟悉的库或功能，必须在修改代码前先使用以下工具进行调研：

1. **查看官方文档**
    - `resolve-library-id` - 解析库名到 Context7 ID
    - `get-library-docs` - 获取最新官方文档

2. **搜索真实代码**
    - `searchGitHub` - 搜索 GitHub 上的实际使用案例
