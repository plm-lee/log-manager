# 项目优化文档

本文档记录了基于 `log-filter-monitor` 项目的优化经验，对 `log-manager` 项目进行的优化改进。

## 优化概览

### 1. 数据库连接优化 ✅

**优化内容：**
- 添加数据库连接池配置（MaxOpenConns, MaxIdleConns, ConnMaxLifetime）
- 添加数据库日志级别配置（silent, error, warn, info）
- 优化数据库初始化流程

**配置文件示例：**
```yaml
database:
  type: "sqlite"
  dsn: "./log_manager.db"
  max_open_conns: 25      # 最大打开连接数
  max_idle_conns: 5        # 最大空闲连接数
  conn_max_lifetime: 300   # 连接最大生存时间（秒）
  log_level: "info"        # 日志级别
```

**性能提升：**
- 减少数据库连接创建开销
- 提高并发处理能力
- 更好的资源管理

### 2. 批量接收接口 ✅

**优化内容：**
- 添加批量接收日志接口：`POST /log/manager/api/v1/logs/batch`
- 添加批量接收指标接口：`POST /log/manager/api/v1/metrics/batch`
- 支持一次性接收最多100条记录
- 使用批量插入提高数据库写入性能

**接口示例：**
```json
POST /log/manager/api/v1/logs/batch
{
  "logs": [
    {
      "timestamp": 1234567890,
      "rule_name": "错误日志",
      "log_line": "ERROR: ...",
      ...
    },
    ...
  ]
}
```

**性能提升：**
- 减少HTTP请求次数
- 提高数据库写入吞吐量（批量插入）
- 降低网络开销

### 3. 请求限流 ✅

**优化内容：**
- 实现基于令牌桶算法的限流中间件
- 支持配置限流速率和桶容量
- 仅对 API 路由生效，不影响健康检查等系统接口

**配置文件示例：**
```yaml
rate_limit:
  enabled: true   # 是否启用限流
  rate: 100       # 每秒允许的请求数
  capacity: 200   # 桶容量（突发请求数）
```

**功能特性：**
- 防止恶意请求和过载
- 保护数据库和系统资源
- 支持突发流量（通过桶容量）

### 4. 优雅关闭机制 ✅

**优化内容：**
- 实现信号处理（SIGINT, SIGTERM）
- 支持优雅关闭HTTP服务器（等待当前请求完成）
- 自动关闭数据库连接
- 5秒超时保护

**实现方式：**
- 使用 `context.WithTimeout` 控制关闭超时
- 使用 `http.Server.Shutdown` 优雅关闭
- 在 `main.go` 中统一管理生命周期

**优势：**
- 避免数据丢失
- 优雅处理正在进行的请求
- 正确释放资源

### 5. 健康检查和监控指标 ✅

**优化内容：**
- 增强健康检查接口：`GET /health`
  - 检查数据库连接状态
  - 返回服务状态和时间戳
- 添加监控指标接口：`GET /metrics`
  - 统计日志和指标条目数量
  - 显示数据库连接池状态（非SQLite）

**接口示例：**
```json
GET /health
{
  "status": "ok",
  "service": "log-manager",
  "time": 1234567890
}

GET /metrics
{
  "service": "log-manager",
  "stats": {
    "log_entries": 1000,
    "metrics_entries": 100,
    "database": {
      "max_open_connections": 25,
      "open_connections": 5,
      "in_use": 2,
      "idle": 3
    }
  }
}
```

### 6. 代码结构优化 ✅

**优化内容：**
- 创建中间件包（`internal/middleware`）
- 优化错误处理
- 改进代码注释和文档
- 统一代码风格

## 性能优化效果

1. **数据库性能：**
   - 连接池减少连接创建开销
   - 批量插入提高写入吞吐量（预计提升 3-5 倍）

2. **网络性能：**
   - 批量接口减少HTTP请求次数
   - 降低网络延迟影响

3. **系统稳定性：**
   - 限流保护系统资源
   - 优雅关闭避免数据丢失
   - 健康检查便于监控

## 与 log-filter-monitor 的对比

| 特性 | log-filter-monitor | log-manager（优化后） |
|------|-------------------|---------------------|
| 模块化设计 | ✅ 高度模块化 | ✅ 模块化结构 |
| 批量处理 | ✅ 支持 | ✅ 已添加 |
| 限流保护 | ⏳ 无 | ✅ 已添加 |
| 优雅关闭 | ✅ 支持 | ✅ 已添加 |
| 健康检查 | ⏳ 无 | ✅ 已添加 |
| 连接池 | N/A | ✅ 已添加 |
| 按标签维度统计 | ✅ 支持 | ⏳ 查询时解析 |
| TCP 长连接接收 | ✅ Agent 支持 | ✅ 默认启用（端口 8890） |
| UDP 接收 | ✅ Agent 支持 | ✅ 可选启用（端口 8889） |
| Web 登录认证 | N/A | ✅ 已添加 |
| 计费统计 | N/A | ✅ 已添加 |

### 7. 数据保留与亿级吞吐优化 ✅

**优化内容：**
- 分批删除 retention（每批 10000 条），避免大事务锁表
- 批量接口独立限流（batch_rate: 1000），支撑每日上亿日志
- 限流双轨：默认 API 100 req/s，批量 API 1000 req/s

**配置示例（亿级场景）：**
```yaml
rate_limit:
  enabled: true
  rate: 100
  batch_rate: 1000
  batch_capacity: 2000

log_retention_days: 30  # 控制单表规模
```

### 8. TCP 长连接接收 ✅

**优化内容：**
- 默认启用 TCP 日志接收（端口 8890），与 HTTP（8888）、UDP（8889）并存
- Agent 默认使用 TCP 长连接，可靠传输 + 高吞吐
- 性能优化：bufio 读写缓冲、sync.Pool 复用 payload、TCP_NODELAY 低延迟、批量落库

**配置示例：**
```yaml
tcp:
  enabled: true
  host: "0.0.0.0"
  port: 8890
  buffer_size: 50000   # 高吞吐缓冲
  flush_interval: "50ms"  # 低延迟
  flush_size: 1000
```

**性能提升：**
- 长连接减少握手开销
- 批量发送 + 批量落库，降低 syscall 和 DB 压力
- 缓冲满时阻塞等待，避免丢包

**MySQL 表分区（可选）：**
日亿级 × 30 天 ≈ 30 亿行时，建议按天分区以加速 retention 清理。需在创建表时执行：

```sql
-- 创建按天分区的 log_entries 表（需在首次部署前执行，已有表需迁移）
CREATE TABLE log_entries (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  timestamp BIGINT NOT NULL,
  rule_name VARCHAR(255),
  rule_desc TEXT,
  log_line TEXT NOT NULL,
  log_file VARCHAR(500),
  pattern TEXT,
  tag VARCHAR(100),
  source VARCHAR(20) DEFAULT 'agent',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  INDEX idx_timestamp (timestamp),
  INDEX idx_tag (tag),
  INDEX idx_rule_name (rule_name)
) PARTITION BY RANGE (TO_DAYS(FROM_UNIXTIME(timestamp))) (
  PARTITION p_min VALUES LESS THAN (0)
);
-- 定期添加新分区、删除旧分区以配合 log_retention_days
```

## 未来优化方向

1. **指标处理优化：**
   - 支持按标签维度存储指标数据
   - 优化指标查询性能
   - 添加指标聚合功能

2. **性能优化：**
   - 添加请求缓存
   - 实现异步写入
   - 优化数据库索引

3. **功能增强：**
   - 添加数据清理任务（基于 log_retention_days）
   - 支持数据导出
   - 添加告警功能

4. **监控和可观测性：**
   - 集成 Prometheus 指标
   - 添加请求追踪
   - 性能分析工具

## 配置建议

### 高并发场景
```yaml
database:
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 300

rate_limit:
  enabled: true
  rate: 200
  capacity: 500
```

### 低资源场景
```yaml
database:
  max_open_conns: 10
  max_idle_conns: 2
  conn_max_lifetime: 300

rate_limit:
  enabled: true
  rate: 50
  capacity: 100
```

## 使用建议

1. **批量接口使用：**
   - 对于高频上报场景，优先使用批量接口
   - 建议批量大小在 50-100 之间，平衡性能和内存

2. **限流配置：**
   - 根据实际负载调整限流参数
   - 监控限流触发情况，及时调整

3. **数据库配置：**
   - SQLite 适合小规模部署
   - 大规模部署建议使用 PostgreSQL 或 MySQL

4. **监控建议：**
   - 定期检查 `/health` 接口
   - 监控 `/metrics` 接口的统计数据
   - 设置告警规则

## 总结

通过借鉴 `log-filter-monitor` 项目的优化经验，我们对 `log-manager` 项目进行了全面的优化：

- ✅ 提升了数据库性能和稳定性
- ✅ 增加了批量处理能力
- ✅ 增强了系统保护机制
- ✅ 改善了系统可观测性
- ✅ 优化了代码结构和可维护性

这些优化使得 `log-manager` 项目能够更好地处理高并发场景，提供更稳定可靠的服务。

