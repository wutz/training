# 部署指南

## 部署到 Kubernetes 集群

### 方式 1: Kustomize 部署

```bash
# 1. 构建镜像
export IMG=ghcr.io/wutz/training:latest
make docker-build docker-push IMG=$IMG

# 2. 部署 operator
make deploy IMG=$IMG

# 3. 验证部署
kubectl get pods -n training-system
kubectl get crd trainingusages.forgeai.wutz.dev
```

### 方式 2: 使用 ko 部署到开发集群

适用于 bz1-dev1 开发集群，无需 Docker daemon：

```bash
# 1. 设置环境变量
export KO_DOCKER_REPO=ghcr.io/wutz
export KUBECONFIG=~/.kube/config-bz1-dev1

# 2. 使用 ko 构建并部署
ko apply -f config/default

# 3. 验证部署
kubectl get pods -n training-system
```

**注意**: bz1-dev1 集群的 Ingress 使用端口 7443，访问格式为 `https://*.dev1.bz1.paratera.com:7443`

### 方式 3: Helm 部署（推荐生产环境）

```bash
# 1. 安装已发布的 chart（chart 版本 = 发布版本去掉 v 前缀，如 v0.2.0 → 0.2.0；镜像默认为 ghcr.io/wutz/training:v0.2.0）
helm install training-operator oci://ghcr.io/wutz/charts/training \
  --version 0.2.0 \
  --namespace training-system \
  --create-namespace

# 2. 升级
helm upgrade training-operator oci://ghcr.io/wutz/charts/training \
  --version 0.3.0 \
  --namespace training-system

# 3. 卸载
helm uninstall training-operator -n training-system
```

**从源码安装**（开发用）：修改 `config/` 后用 `kubebuilder edit --plugins=helm/v2-alpha` 重新生成 `dist/chart` 并提交，然后：

```bash
helm install training-operator ./dist/chart \
  --namespace training-system \
  --create-namespace \
  --set manager.image.repository=ghcr.io/wutz/training \
  --set manager.image.tag=latest
```

## 配置选项

### 环境变量

在 `config/manager/manager.yaml` 中配置：

```yaml
env:
- name: ENABLE_WEBHOOKS
  value: "false"
- name: METRICS_ADDR
  value: ":8080"
- name: HEALTH_PROBE_ADDR
  value: ":8081"
- name: LEADER_ELECT
  value: "true"
```

### 资源限制

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### Replica 配置

生产环境建议启用高可用：

```yaml
replicas: 2
```

## 前置条件检查

部署前确认以下组件已安装：

```bash
# 1. 检查 Kubeflow Training Operator
kubectl get crd trainjobs.trainer.kubeflow.org

# 2. 检查 CRD API 版本
kubectl get trainjob -A  # 应该返回 v1alpha1 版本

# 3. 验证 RBAC 权限
kubectl auth can-i create trainingusages --as=system:serviceaccount:training-system:training-controller-manager
```

## 开发环境部署

### 本地 Kind 集群

```bash
# 1. 创建 Kind 集群
kind create cluster --name training-dev

# 2. 加载镜像（避免推送到 registry）
make docker-build IMG=training-operator:dev
kind load docker-image training-operator:dev --name training-dev

# 3. 部署
make deploy IMG=training-operator:dev

# 4. 端口转发（访问 metrics）
kubectl port-forward -n training-system svc/training-controller-manager-metrics-service 8080:8443
```

### k3d 集群（Mac 推荐）

**注意**: 由于 Kind 在某些 Mac 上无法启动节点，推荐使用 k3d

```bash
# 1. 创建 k3d 集群
k3d cluster create training-dev \
  --registry-create training-registry:0.0.0.0:5000

# 2. 构建并推送到本地 registry
export KO_DOCKER_REPO=localhost:5000
ko apply -f config/default

# 3. 验证
kubectl get pods -n training-system -w
```

### vcluster 开发（在 bz1-dev1 上）

在 bz1-dev1 集群上创建虚拟集群：

```bash
# 1. 创建 vcluster
vcluster create training-dev -n vcluster-training

# 2. 连接到 vcluster
vcluster connect training-dev -n vcluster-training

# 3. 在 vcluster 中部署
make deploy IMG=ghcr.io/wutz/training:dev

# 4. 断开连接
vcluster disconnect
```

## 升级和回滚

### 升级到新版本

```bash
# 1. 更新 CRD
make install

# 2. 更新 controller
export IMG=ghcr.io/wutz/training:v0.2.0
make docker-build docker-push IMG=$IMG
kubectl set image deployment/training-controller-manager \
  manager=$IMG \
  -n training-system

# 3. 验证升级
kubectl rollout status deployment/training-controller-manager -n training-system
```

### 回滚

```bash
# 查看历史版本
kubectl rollout history deployment/training-controller-manager -n training-system

# 回滚到上一版本
kubectl rollout undo deployment/training-controller-manager -n training-system

# 回滚到指定版本
kubectl rollout undo deployment/training-controller-manager \
  --to-revision=2 \
  -n training-system
```

## 卸载

```bash
# 1. 删除 controller
make undeploy

# 2. 删除 CRD（会删除所有 TrainingUsage 资源）
kubectl delete crd trainingusages.forgeai.wutz.dev

# 3. 删除 namespace（可选）
kubectl delete namespace training-system
```

**警告**: 删除 CRD 会永久删除所有 TrainingUsage 记录，请先导出计费数据！

## 生产环境最佳实践

1. **启用 Leader Election** - 多副本部署时避免竞争
2. **配置 PodDisruptionBudget** - 保证高可用
3. **设置资源请求和限制** - 防止资源耗尽
4. **启用 Prometheus 监控** - 实时追踪指标
5. **定期备份 TrainingUsage 数据** - 防止数据丢失
6. **使用镜像标签而非 latest** - 确保版本可控

## 故障排查

### Controller 启动失败

```bash
# 查看日志
kubectl logs -n training-system deployment/training-controller-manager

# 常见问题：
# 1. RBAC 权限不足
kubectl describe clusterrole training-manager-role

# 2. CRD 未安装
kubectl get crd trainingusages.forgeai.wutz.dev

# 3. 镜像拉取失败
kubectl describe pod -n training-system -l control-plane=controller-manager
```

### TrainingUsage 未创建

```bash
# 1. 检查 TrainJob 是否存在
kubectl get trainjobs -A

# 2. 检查 controller 日志
kubectl logs -n training-system deployment/training-controller-manager | grep TrainingUsage

# 3. 手动触发 reconcile
kubectl annotate trainjob my-job force-sync=true
```

### 计量数据不准确

```bash
# 1. 检查 Pod 状态
kubectl get pods -n <namespace> -l job-set-name=<trainjob>

# 2. 检查 TrainingUsage 状态
kubectl describe trainingusage <name> -n <namespace>

# 3. 验证时间戳
kubectl get trainingusage <name> -n <namespace> -o jsonpath='{.status.startTime}'
```

## 监控和告警

### Prometheus 配置

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: training-controller-manager
  namespace: training-system
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  endpoints:
  - port: https
    scheme: https
    interval: 30s
```

### Grafana Dashboard

导入 dashboard JSON：

```bash
kubectl create configmap training-dashboard \
  --from-file=dashboard.json \
  -n monitoring
```
