# BENZHI_README

## 项目说明

- 项目：11DingKing/goZZS-04
- 项目用途：A production-grade Go backend for nature-reserve patrol dispatch, incident management, emergency resource allocation, material outbound, and offline report synchronization.
- Go 工具链：`golang:1.26`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/server

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-9-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-9-arm64 linux/arm64
docker run -it benzhi-task-9-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-9-arm64:latest
```

## 题目验证命令

1. 预期退出码 1：`go test ./internal/transport/http/ -run "TestHTTP_DispatchStaysAvailableAfterDuplicateRelease|TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest|TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest|TestHTTP_ReleaseReturnsCapacityToThePool" -count=1 -v`

## Bug 复现

Bug 现象、触发步骤和完整错误信息见 `BUG_REPRO.md`。
