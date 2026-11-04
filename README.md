# Training Operator

训练任务计量 Operator，为 Kubeflow TrainJob 提供资源使用追踪和计费支持。

## 概述

Training Operator 通过 `TrainingUsage` CRD 自动追踪每个训练任务的资源消耗（CPU、内存、GPU），为计费系统提供准确的用量数据。

### 核心功能

- **自动计量** - 训练任务启动时自动创建 TrainingUsage 资源
- **实时追踪** - 每分钟更新资源使用量，基于时间积分计算
- **重试聚合** - 自动累加重试任务的资源消耗
- **生命周期管理** - 追踪从运行到完成/失败的完整状态

## 架构

### CRD 定义

```go
type TrainingUsageSpec struct {
    TrainJob     string                      // 关联的 TrainJob 名称
    InstanceType string                      // 实例类型
    Workers      int32                       // worker 数量
    Resources    corev1.ResourceRequirements // 每个 worker 的资源请求
}

type TrainingUsageStatus struct {
    Phase          UsagePhase      // 计费阶段：Active/Completed/Failed/Exported
    StartTime      *metav1.Time    // 开始时间
    EndTime        *metav1.Time    // 结束时间
    Usage          *ResourceUsage  // 累计资源消耗
    LastUpdateTime *metav1.Time    // 最后更新时间
    Attempts       int32           // 重试次数
}
```

### 计量指标

所有指标基于**时间积分**计算，确保公平计费：

- **CPU-秒**: `CPU核心数 × worker数量 × 运行时长（秒）`
- **内存-字节-秒**: `内存字节数 × worker数量 × 运行时长（秒）`
- **GPU-秒**: `GPU数量 × worker数量 × 运行时长（秒）`

### Controller 逻辑

```
┌─────────────────────────────────────────────────────┐
│  TrainingUsageReconciler                            │
├─────────────────────────────────────────────────────┤
│  1. 监听 TrainJob 和 TrainingUsage 变化             │
│  2. 获取关联的 TrainJob 和 Pods                     │
│  3. 计算自上次更新以来的资源增量                     │
│  4. 累加到 status.usage                             │
│  5. Active 任务每分钟重新入队                       │
└─────────────────────────────────────────────────────┘
```

## 快速开始

### 前置条件

- Kubernetes 集群 (v1.28+)
- Kubeflow Training Operator v2 已安装
- kubectl 和 kustomize

### 安装 CRD

```bash
# 安装 CRD
kubectl apply -f config/crd/bases/forgeai.wutz.dev_trainingusages.yaml

# 部署 operator
export IMG=ghcr.io/wutz/training:latest
make docker-build docker-push IMG=$IMG
make deploy IMG=$IMG
```

### 验证安装

```bash
# 检查 CRD
kubectl get crd trainingusages.forgeai.wutz.dev

# 检查 controller
kubectl get pods -n training-system

# 查看示例
kubectl apply -f config/samples/forgeai_v1alpha1_trainingusage.yaml
kubectl get trainingusage
```

## 使用示例

### 查看计量记录

```bash
# 查看所有计量
kubectl get trainingusages -A
# 或简写
kubectl get tu -A

# 查看详情
kubectl describe tu my-job-usage -n my-project
```

### 导出计费数据

```bash
# JSON 格式
kubectl get tu -A -o json | jq '.items[] | select(.status.phase == "Completed")'

# CSV 格式
kubectl get tu -A -o json | jq -r '
  ["Namespace","Job","Workers","CPU(s)","GPU(s)"],
  (.items[] | [
    .metadata.namespace,
    .spec.trainJob,
    .spec.workers,
    .status.usage.cpu,
    .status.usage.gpu
  ])
  | @csv
'
```

## 开发

### 本地开发

```bash
# 安装依赖
go mod download

# 生成代码
make manifests generate

# 运行测试
make test

# 本地运行（需要 kubeconfig）
make run
```

### 修改 CRD

编辑 `api/v1alpha1/trainingusage_types.go` 后：

```bash
make manifests generate
make test
```

## 集成计费系统

### 方案 1: 定期轮询

计费系统定期查询已完成的 TrainingUsage：

```python
from kubernetes import client, config

config.load_kube_config()
api = client.CustomObjectsApi()

usages = api.list_cluster_custom_object(
    group="forgeai.wutz.dev",
    version="v1alpha1",
    plural="trainingusages"
)

for item in usages['items']:
    if item['status']['phase'] in ['Completed', 'Failed']:
        # 提取计费数据并提交到计费系统
        # ...
```

### 方案 2: Webhook 通知

创建 webhook controller 监听状态变化，实时推送到计费系统。

## 配置

### 环境变量

- `ENABLE_WEBHOOKS` - 启用 webhook（默认: false）
- `METRICS_ADDR` - Prometheus metrics 地址（默认: :8080）
- `HEALTH_PROBE_ADDR` - 健康检查地址（默认: :8081）

### RBAC

Operator 需要以下权限：

```yaml
- trainingusages: get, list, watch, create, update, patch, delete
- trainjobs: get, list, watch
- pods: get, list, watch
```

## 监控

### Prometheus 指标

```promql
# GPU 使用总量（秒）
forgeai_training_usage_gpu_seconds_total

# 项目成本
sum(forgeai_training_usage_gpu_seconds_total{phase="Completed"}) by (namespace)
```

## 故障排查

### 计量数据不准确

```bash
# 检查 controller 日志
kubectl logs -n training-system deployment/training-controller-manager

# 验证 Pod 时间戳
kubectl get pods -n <namespace> -l job-set-name=<trainjob> \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.startTime}{"\n"}{end}'
```

### TrainingUsage 未创建

确认：
1. TrainJob 是否包含所需 labels 和 annotations
2. Controller 是否正常运行
3. RBAC 权限是否正确

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

Apache License 2.0
