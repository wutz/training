# 开发指南

## 开发环境设置

### 前置要求

- Go 1.23+
- kubectl
- kubebuilder v4.x
- Docker 或 ko
- 访问 Kubernetes 集群

### 克隆并初始化

```bash
git clone https://github.com/wutz/training.git
cd training
go mod download
```

### 本地运行

```bash
# 方式 1: 直接运行（需要 kubeconfig）
make run

# 方式 2: 调试模式
dlv debug cmd/main.go
```

### 代码生成

修改 CRD 定义后重新生成：

```bash
# 生成 manifests 和 deepcopy
make manifests generate

# 只生成 CRD
make manifests

# 只生成 deepcopy
make generate
```

## 项目结构

```
training/
├── api/v1alpha1/           # CRD 定义
│   ├── trainingusage_types.go
│   └── zz_generated.deepcopy.go
├── internal/controller/    # Controller 逻辑
│   └── trainingusage_controller.go
├── pkg/                    # 共享包
│   ├── kf/                 # Kubeflow 类型
│   └── naming/             # 常量和命名
├── config/                 # Kustomize 配置
│   ├── crd/                # CRD manifests
│   ├── manager/            # Controller 部署
│   ├── rbac/               # RBAC 权限
│   └── samples/            # 示例资源
├── cmd/main.go             # 入口
└── docs/                   # 文档
```

## 测试

### 单元测试

```bash
# 运行所有测试
make test

# 运行特定测试
go test ./internal/controller -v -run TestTrainingUsage

# 查看覆盖率
make test-coverage
```

### 集成测试

```bash
# 需要 Kind 集群
make test-e2e
```

### 本地验证

```bash
# 1. 启动本地集群
kind create cluster --name training-test

# 2. 安装 CRD
make install

# 3. 运行 controller
make run

# 4. 在另一个终端创建测试资源
kubectl apply -f config/samples/
```

## 调试

### 查看日志

```bash
# Controller 日志
kubectl logs -n training-system deployment/training-controller-manager -f

# 带时间戳
kubectl logs -n training-system deployment/training-controller-manager -f --timestamps

# 过滤特定资源
kubectl logs -n training-system deployment/training-controller-manager -f | grep TrainingUsage
```

### 启用详细日志

在 `config/manager/manager.yaml` 中：

```yaml
args:
- --leader-elect
- --zap-log-level=debug  # 添加此行
```

### 使用 Delve 调试

```bash
# 安装 delve
go install github.com/go-delve/delve/cmd/dlv@latest

# 启动调试会话
dlv debug cmd/main.go -- --leader-elect=false

# 在 delve 中
(dlv) break internal/controller/trainingusage_controller.go:65
(dlv) continue
```

## 代码规范

### Linting

```bash
# 运行 linter
make lint

# 自动修复
make lint-fix
```

### 提交规范

遵循 Conventional Commits：

```
feat: 新功能
fix: 修复 bug
docs: 文档更新
refactor: 重构
test: 测试相关
chore: 构建/工具相关
```

示例：
```bash
git commit -m "feat: add GPU memory tracking to TrainingUsage"
git commit -m "fix: correct duration calculation for multi-retry jobs"
```

## 常见开发任务

### 添加新字段到 CRD

1. 编辑 `api/v1alpha1/trainingusage_types.go`：

```go
type TrainingUsageStatus struct {
    // 现有字段...
    
    // 新字段
    // +optional
    NetworkBytes *resource.Quantity `json:"networkBytes,omitempty"`
}
```

2. 添加 kubebuilder 标记（如需要）：

```go
// +kubebuilder:printcolumn:name="Network",type=string,JSONPath=`.status.networkBytes`
```

3. 重新生成：

```bash
make manifests generate
```

4. 更新 controller 逻辑处理新字段

5. 添加测试

### 修改 Controller 逻辑

1. 编辑 `internal/controller/trainingusage_controller.go`
2. 运行测试：`make test`
3. 本地验证：`make run`
4. 提交前检查：`make lint-fix && make test`

### 添加新的 CRD

```bash
# 使用 kubebuilder 脚手架
kubebuilder create api \
  --group forgeai \
  --version v1alpha1 \
  --kind NewResource \
  --resource \
  --controller

# 编辑生成的文件
# 重新生成
make manifests generate
```

## 性能优化

### Profiling

```go
import _ "net/http/pprof"
import "net/http"

go func() {
    http.ListenAndServe("localhost:6060", nil)
}()
```

访问 `http://localhost:6060/debug/pprof/`

### 减少 API 调用

使用 cache 和 informer：

```go
// 好的做法
pods := &corev1.PodList{}
r.List(ctx, pods, client.InNamespace(ns), client.MatchingLabels{...})

// 避免在循环中调用 Get
for _, pod := range pods.Items {
    // 直接使用 pod
}
```

### 优化 Reconcile 频率

```go
// 对于不需要频繁更新的资源
if usage.Status.Phase != UsagePhaseActive {
    return ctrl.Result{}, nil  // 不重新入队
}

// 活跃资源使用较长间隔
return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
```

## 构建和发布

### 构建镜像

```bash
# 使用 Docker
export IMG=ghcr.io/wutz/training:v0.1.0
make docker-build docker-push IMG=$IMG

# 使用 ko（推荐，更快）
export KO_DOCKER_REPO=ghcr.io/wutz
ko build -B cmd/main.go -t v0.1.0
```

### 版本发布流程

1. 更新版本号：
   - `PROJECT` 文件
   - `config/manager/kustomization.yaml`
   - README.md

2. 创建 tag：
```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

3. 构建并推送镜像：
```bash
make release IMG=ghcr.io/wutz/training:v0.1.0
```

4. 创建 GitHub Release

## 故障排查

### Controller 启动失败

```bash
# 检查 RBAC
kubectl auth can-i create trainingusages \
  --as=system:serviceaccount:training-system:training-controller-manager

# 检查 CRD
kubectl get crd trainingusages.forgeai.wutz.dev -o yaml

# 查看事件
kubectl get events -n training-system --sort-by='.lastTimestamp'
```

### 测试失败

```bash
# 清理测试环境
make clean

# 重新生成 manifests
make manifests generate

# 重新运行
make test
```

### 本地开发问题

**GOWORK 冲突**：
```bash
# 如果主项目使用 go.work，在此仓库中禁用
export GOWORK=off
make generate manifests
```

**Kind 节点启动失败（Mac）**：
```bash
# 使用 k3d 代替
k3d cluster create training-dev
```

## 贡献流程

1. Fork 仓库
2. 创建 feature 分支：`git checkout -b feature/my-feature`
3. 提交更改：`git commit -am 'feat: add new feature'`
4. 推送分支：`git push origin feature/my-feature`
5. 创建 Pull Request

### PR 检查清单

- [ ] 代码通过 `make lint`
- [ ] 所有测试通过 `make test`
- [ ] 添加了必要的测试
- [ ] 更新了文档
- [ ] Commit 消息符合规范
- [ ] 添加了 `Co-Authored-By: Claude Code <noreply@anthropic.com>`

## 有用的资源

- [Kubebuilder Book](https://book.kubebuilder.io)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
