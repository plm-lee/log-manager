# 日志管理系统 (Log Manager)

日志管理系统用于接收和管理来自 `log-filter-monitor` 上报的日志和指标数据，提供统一的管理后台界面。

## 功能特性

- 📝 **日志接收与管理**：接收来自多个项目的日志数据，支持按标签、规则名称、关键词、时间范围等条件查询
- 📊 **指标统计展示**：展示各项目的指标统计数据，包括规则匹配计数、总计数等
- 🏷️ **标签分类**：通过标签区分不同项目上报的日志和指标
- 🔍 **强大的查询功能**：支持多条件组合查询，快速定位目标日志
- 🎨 **现代化 UI**：基于 React + Ant Design 构建的美观易用的管理界面

## 项目结构

```
log-manager/
├── backend/              # 后端服务（Go）
│   ├── config.yaml      # 配置文件
│   ├── go.mod           # Go 依赖管理
│   ├── main.go          # 入口文件
│   └── internal/
│       ├── app/         # 应用初始化
│       ├── config/      # 配置管理
│       ├── database/    # 数据库连接
│       ├── handler/     # HTTP 处理器
│       └── models/      # 数据模型
└── frontend/            # 前端应用（React）
    ├── package.json     # 依赖管理
    ├── public/          # 静态资源
    └── src/
        ├── api/         # API 接口封装
        ├── components/  # 公共组件
        ├── pages/       # 页面组件
        ├── App.js       # 应用主组件
        └── index.js     # 入口文件
```

## 快速开始

### 一键启动

在项目根目录执行：

```bash
./start.sh          # 开发：前台运行后端+前端，Ctrl+C 停止
./start.sh -d       # 开发：后台运行，输出写入 logs/，使用 ./stop.sh 停止
./start.sh build    # 打包前端到 backend/web，用于生产部署
./start.sh prod     # 生产：仅启动后端（托管前端），单端口 8080，无需 Node.js
./start.sh prod -d  # 生产：后台运行
```

开发模式会启动后端（8080）和前端开发服务器（3000）；生产模式仅启动后端，将前端静态文件一并托管，适合服务器部署。

### 生产部署流程

1. 本地或 CI 执行：`./start.sh build` 打包前端
2. 将 `backend/` 目录（含 `web/`、`config.prod.yaml`）上传至服务器
3. 服务器执行：`cd backend && CONFIG=config.prod.yaml go run main.go` 或使用 `./start.sh prod`
4. 访问 http://服务器:8080 即可使用

### 手动启动

**后端**：`cd backend && go run main.go`（端口 8080）

**前端开发**：`cd frontend && npm install && npm start`（端口 3000）

### 配置 log-filter-monitor

在 `log-filter-monitor` 的配置文件中，设置 HTTP 处理器：

```yaml
handler:
  type: http
  api_url: http://localhost:8080/api/v1/logs
  timeout: 10s
```

## API 接口

### 日志接口

#### 接收日志
- **POST** `/api/v1/logs`
- 接收来自 log-filter-monitor 的日志上报

#### 查询日志
- **GET** `/api/v1/logs`
- 查询参数：
  - `tag`: 标签筛选
  - `rule_name`: 规则名称筛选
  - `keyword`: 关键词搜索（在日志内容中搜索）
  - `start_time`: 开始时间戳
  - `end_time`: 结束时间戳
  - `page`: 页码（从1开始）
  - `page_size`: 每页数量

#### 获取标签列表
- **GET** `/api/v1/logs/tags`

#### 获取规则名称列表
- **GET** `/api/v1/logs/rule-names`

### 指标接口

#### 接收指标
- **POST** `/api/v1/metrics`
- 接收来自 log-filter-monitor 的指标上报

#### 查询指标
- **GET** `/api/v1/metrics`
- 查询参数：
  - `tag`: 标签筛选
  - `start_time`: 开始时间戳
  - `end_time`: 结束时间戳
  - `page`: 页码（从1开始）
  - `page_size`: 每页数量

## 配置说明

### 后端配置 (config.yaml)

```yaml
server:
  host: "0.0.0.0"      # 监听地址
  port: 8080           # 监听端口

database:
  type: "sqlite"       # 数据库类型
  dsn: "./log_manager.db"  # 数据库连接字符串

log_retention_days: 30  # 日志保留天数（0 表示永久保留）

cors:
  enabled: true
  allow_origins:
    - "http://localhost:3000"
  allow_methods:
    - GET
    - POST
    - PUT
    - DELETE
    - OPTIONS
  allow_headers:
    - Content-Type
    - Authorization
```

## 使用说明

1. **查看日志**：
   - 在日志列表页面，可以通过标签、规则名称、关键词、时间范围等条件筛选日志
   - 支持分页浏览和每页数量调整

2. **查看指标**：
   - 在指标统计页面，可以查看各项目的指标数据
   - 点击展开按钮可以查看详细的规则计数信息

3. **标签管理**：
   - 系统会自动从上报的日志中提取标签
   - 通过标签可以快速筛选特定项目的日志和指标

## 开发说明

### 后端技术栈
- Go 1.21+
- Gin (Web 框架)
- GORM (ORM 框架)
- SQLite (数据库)

### 前端技术栈
- React 18
- Ant Design 5
- React Router 6
- Axios
- Day.js

## 许可证

MIT License
