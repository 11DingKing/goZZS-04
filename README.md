# 保护区巡护调度系统 (Patrol Dispatch Service)

A production-grade Go backend for nature-reserve patrol dispatch, incident
management, emergency resource allocation, material outbound, and offline
report synchronization.

## 业务概述

调度员通过统一平台向管护网格的巡护员派发巡护任务，巡护员在责任区
签到并回传轨迹与影像。发现火情、盗猎或越界等异常时上报定位与证据，
调度员复核后调配无人机、巡护车辆等应急资源，物资管理员办理出库，
处置结果归档。

核心业务规则：

- 巡护员须在责任网格签到，漏签需调度员二次确认
- 异常上报必须附带定位与影像，缺失则退回补正
- 应急资源调配须经调度员复核后方可执行
- 物资出库须领用人与物资管理员双签
- 火情等高危事件须在十分钟内响应，超时自动升级至保护区管理局
- 多组巡护组同时申领同一车辆或物资时，按事件优先级与申请时间裁决
- 通信中断时终端本地缓存，恢复后按上报时间顺序补传，幂等防重

## 技术栈

- Go 1.26（仅标准库，零外部依赖）
- HTTP JSON API（Go 1.22+ 路由模式）
- 线程安全内存存储
- 后台任务：超时升级巡检、离线补传

## 项目结构

```
cmd/server/main.go              程序入口，HTTP 服务与后台任务编排
internal/config/                 配置加载与默认值
internal/domain/                 领域模型与状态机（patrol, incident, resource, material, sync）
internal/store/                  线程安全持久化层
internal/service/                应用编排（dispatch, resource, material, sync）
internal/transport/http/         HTTP 接入层（handler, router）
internal/worker/                 后台任务（escalation, sync replay）
```

## 启动方式

```bash
# 直接运行（默认监听 :53649）
go run ./cmd/server

# 指定配置文件
PATROL_CONFIG=/path/to/config.json go run ./cmd/server
```

服务启动后监听 `http://localhost:53649`。

## 主要接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/actors/grids` | 注册管护网格 |
| POST | `/api/v1/actors/officers` | 注册巡护员 |
| POST | `/api/v1/actors/dispatchers` | 注册调度员 |
| POST | `/api/v1/actors/managers` | 注册物资管理员 |
| POST | `/api/v1/tasks` | 派发巡护任务 |
| POST | `/api/v1/tasks/{id}/sign-in` | 网格签到 |
| POST | `/api/v1/tasks/{id}/confirm-missed` | 调度员补签 |
| POST | `/api/v1/tasks/{id}/trajectory` | 回传轨迹 |
| POST | `/api/v1/tasks/{id}/complete` | 完成巡护 |
| POST | `/api/v1/incidents` | 上报异常（须含定位+影像） |
| POST | `/api/v1/incidents/{id}/review` | 调度员复核 |
| POST | `/api/v1/incidents/{id}/dispatch` | 启动资源调配 |
| POST | `/api/v1/incidents/{id}/respond` | 响应到达（满足10分钟规则） |
| POST | `/api/v1/incidents/{id}/resolve` | 处置归档 |
| POST | `/api/v1/resources` | 登记应急资源 |
| POST | `/api/v1/resources/{id}/requests` | 申领资源 |
| POST | `/api/v1/resource-requests/{id}/approve` | 调度员复核批准 |
| POST | `/api/v1/resource-requests/{id}/allocate` | 分配资源 |
| POST | `/api/v1/resources/{id}/arbitrate` | 优先级裁决 |
| POST | `/api/v1/resource-requests/{id}/release` | 释放资源 |
| POST | `/api/v1/materials` | 登记物资 |
| POST | `/api/v1/material-requests` | 创建出库申请（领用人签） |
| POST | `/api/v1/material-requests/{id}/sign` | 物资管理员双签完成 |
| POST | `/api/v1/material-requests/arbitrate` | 物资优先级裁决 |
| POST | `/api/v1/sync/cache` | 缓存离线上报 |
| POST | `/api/v1/sync/replay` | 按时间顺序补传 |
| GET  | `/api/v1/health` | 健康检查 |

## 端口

默认监听 `53649`，可通过 `config.json` 的 `listen_addr` 修改。

## 测试方法

```bash
# 运行全部测试
go test -timeout=120s -count=1 ./...

# 运行特定包测试
go test -v ./internal/service/...

# 运行单个测试
go test -v -run TestResourceArbitration ./internal/service/
```

测试覆盖正常路径、错误路径、状态迁移、并发裁决与取消、以及失败恢复，
全部不依赖外部服务。

## Docker

```bash
# 构建镜像
docker build -t patrol-dispatch .

# 运行容器
docker run -p 53649:53649 patrol-dispatch

# 多架构构建（amd64 + arm64）
docker buildx build --platform linux/amd64,linux/arm64 -t patrol-dispatch:multi .
```

构建使用 `golang:1.26` 多阶段构建，最终镜像仅包含编译后的二进制文件和
配置文件，支持 amd64 与 arm64 架构。
