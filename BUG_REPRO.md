# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

帮我查一个应急资源模块整体卡死的问题。先不要改代码，我需要先拿到准确的根因和证据再决定怎么动。

现象：

1. 调度台上有人把同一笔资源申请的「释放」点了两次。第二次 POST /api/v1/resource-requests/{id}/release 返回 400 cannot release: request status is released，看起来只是一个正常的重复操作提示。
2. 但从那一刻起，应急资源相关的接口全部不再返回：POST /api/v1/resource-requests/{id}/allocate、POST /api/v1/resource-requests/{id}/release、POST /api/v1/resources/{id}/arbitrate 发过去就一直挂着，不返回结果、不报错、也不超时，最后是客户端自己断开。
3. 服务进程还活着：GET /api/v1/health 正常 200；巡护任务、异常上报、物资出库这些模块也都正常。只有应急资源这一块整块卡住。
4. 另外两种操作会触发同样的结果：释放一个不存在的申请 ID（返回 400 resource request ... not found），或者释放一个已批准但还没分配的申请（返回 400 cannot release: request status is approved）。
5. 只要释放调用是成功返回 200 的，后续一切正常，能继续分配、能继续裁决。
6. 重启服务能恢复，但只要再出现一次失败的释放调用，又会卡住。资源的 in_use / capacity 数据本身看起来没有被写坏。

复现：分配一笔资源，正常释放一次（200），再释放同一笔（400），然后随便发一个分配或裁决请求，观察它是否还会返回。

请定位这个「一次失败的释放之后整块资源接口不再响应」的根因：说明是哪个 Go 文件里的哪个符号、它的什么错误行为，以及这个错误行为为什么会造成上面这些症状（包括为什么成功释放不会触发、为什么只影响资源模块、为什么进程还活着）。先给结论和证据，不要改仓库里的代码。

## 含 Bug 版本

- 仓库：11DingKing/goZZS-04
- 仓库地址：https://github.com/11DingKing/goZZS-04.git
- parent SHA：bad29ac61985230a3b13f9b581b2208205cfc12a

## 复现步骤

```bash
git clone -- https://github.com/11DingKing/goZZS-04.git bug-repro
cd bug-repro
git checkout --detach bad29ac61985230a3b13f9b581b2208205cfc12a
go test ./internal/transport/http/ -run "TestHTTP_DispatchStaysAvailableAfterDuplicateRelease|TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest|TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest|TestHTTP_ReleaseReturnsCapacityToThePool" -count=1 -v
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/transport/http/ -run "TestHTTP_DispatchStaysAvailableAfterDuplicateRelease|TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest|TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest|TestHTTP_ReleaseReturnsCapacityToThePool" -count=1 -v
=== RUN   TestHTTP_DispatchStaysAvailableAfterDuplicateRelease
    resource_release_test.go:116: allocate after duplicate release: POST /api/v1/resource-requests/req_44a065b68e42_3/allocate did not respond within 5s
--- FAIL: TestHTTP_DispatchStaysAvailableAfterDuplicateRelease (5.02s)
=== RUN   TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest
    resource_release_test.go:142: allocate after unknown release: POST /api/v1/resource-requests/req_5245516ee402_5/allocate did not respond within 5s
--- FAIL: TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest (5.00s)
=== RUN   TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest
    resource_release_test.go:162: allocate after unallocated release: POST /api/v1/resource-requests/req_a68896619631_7/allocate did not respond within 5s
--- FAIL: TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest (5.01s)
=== RUN   TestHTTP_ReleaseReturnsCapacityToThePool
--- PASS: TestHTTP_ReleaseReturnsCapacityToThePool (0.00s)
FAIL
FAIL	github.com/reserve/patrol-dispatch/internal/transport/http	15.077s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/transport/http/ -run "TestHTTP_DispatchStaysAvailableAfterDuplicateRelease|TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest|TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest|TestHTTP_ReleaseReturnsCapacityToThePool" -count=1 -v
=== RUN   TestHTTP_DispatchStaysAvailableAfterDuplicateRelease
    resource_release_test.go:116: allocate after duplicate release: POST /api/v1/resource-requests/req_2744be94b7d5_3/allocate did not respond within 5s
--- FAIL: TestHTTP_DispatchStaysAvailableAfterDuplicateRelease (5.01s)
=== RUN   TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest
    resource_release_test.go:142: allocate after unknown release: POST /api/v1/resource-requests/req_b7184f6cf1b5_5/allocate did not respond within 5s
--- FAIL: TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest (5.01s)
=== RUN   TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest
    resource_release_test.go:162: allocate after unallocated release: POST /api/v1/resource-requests/req_d137a96b7149_7/allocate did not respond within 5s
--- FAIL: TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest (5.01s)
=== RUN   TestHTTP_ReleaseReturnsCapacityToThePool
--- PASS: TestHTTP_ReleaseReturnsCapacityToThePool (0.00s)
FAIL
FAIL	github.com/reserve/patrol-dispatch/internal/transport/http	15.035s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

通过标准（diagnosis）：
1. 命中 gold 根因涉及的文件：internal/service/resource.go
2. 命中 gold 根因涉及的符号：(*ResourceService).ReleaseResource
3. 命中正确的失效机制：该方法取得 allocMu 之后没有用 defer 保证释放，只在成功路径末尾解锁，因此三条 return err 的提前返回路径都会带着已加锁的互斥量退出；allocMu 被永久持有后，所有同样先 Lock 该互斥量的资源操作（AllocateResource、ArbitrateResource、ReleaseResource）都在 Lock 处永久阻塞，而不走该锁的模块与健康检查不受影响
4. 结论有实际证据（读过相关代码或跑过复现），不是凭空推断
5. 目标仓库全程零改动；容器内一次性独立复现程序不计为项目代码改动
6. 复现依据：go test ./internal/transport/http/ -run 'TestHTTP_DispatchStaysAvailableAfterDuplicateRelease|TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest|TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest|TestHTTP_ReleaseReturnsCapacityToThePool' -count=1 在 main 上失败、在 gold_model_fix 上通过
