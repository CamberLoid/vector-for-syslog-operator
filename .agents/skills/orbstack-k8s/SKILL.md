---
name: orbstack-k8s
description: Orbstack 本地 Kubernetes 环境管理。当用户需要在 macOS 上使用本地 Kubernetes 集群进行开发测试、需要配置 kubectl 访问本地集群、需要调试 Kubernetes 资源或排查集群问题时使用此 skill。
---

# Orbstack Kubernetes 本地开发环境

本 skill 涵盖使用 Orbstack 作为本地 Kubernetes 开发环境的配置、使用方法和常见操作。

---

## 概述

Orbstack 是 macOS 上的轻量级容器和 Kubernetes 运行环境，相比 Docker Desktop 和 Minikube：

- **启动速度快** - 几秒内启动 Kubernetes
- **资源占用低** - 更少的 CPU 和内存消耗
- **网络性能优** - 本地服务访问更流畅
- **域名自动解析** - `*.orb.local` 自动解析到服务

---

## 前置要求

- macOS 11+ (Apple Silicon 或 Intel)
- Orbstack 已安装
- kubectl 已安装

---

## 常用命令

### 集群管理

| 命令 | 说明 |
|------|------|
| `orb start k8s` | 启动 Kubernetes 集群 |
| `orb stop k8s` | 停止 Kubernetes 集群 |
| `orb delete k8s` | 删除 Kubernetes 集群 |
| `orb status` | 查看 Orbstack 状态 |
| `orb update` | 更新 Orbstack |

### kubectl 上下文

```bash
# 查看当前上下文
kubectl config current-context

# 切换到 Orbstack 集群
kubectl config use-context orbstack

# 查看所有上下文
kubectl config get-contexts

# 设置默认命名空间
kubectl config set-context --current --namespace=default
```

### 集群信息

```bash
# 查看集群节点
kubectl get nodes -o wide

# 查看集群版本
kubectl version

# 查看集群信息
kubectl cluster-info
```

---

## 网络配置

### 服务访问

**方式 1: ClusterIP + kubectl proxy**

```bash
# 本地端口转发
kubectl port-forward svc/my-service 8080:80

# 在另一个终端访问
curl http://localhost:8080
```

**方式 2: LoadBalancer (自动分配)**

```bash
# Orbstack 自动为 LoadBalancer 服务分配 IP
kubectl get svc
# NAME         TYPE           CLUSTER-IP      EXTERNAL-IP    PORT(S)
# my-service   LoadBalancer   10.43.123.45    192.168.215.5  80:30080/TCP

# 直接访问
# curl http://192.168.215.5
```

**方式 3: NodePort**

```bash
# 使用节点的 IP 和 NodePort
kubectl get nodes -o wide
# NAME       STATUS   EXTERNAL-IP
# orbstack   Ready    192.168.215.2

# 访问 NodePort 服务
# curl http://192.168.215.2:30080
```

**方式 4: Ingress + orb.local 域名**

```bash
# 创建 Ingress 资源
# 自动解析为 service-name.namespace.orb.local
curl http://my-service.default.orb.local
```

### 域名自动解析

Orbstack 支持 `*.orb.local` 自动解析：

```bash
# 服务默认域名格式
<service-name>.<namespace>.orb.local

# 示例
kubectl create deployment nginx --image=nginx
kubectl expose deployment nginx --port=80

# 访问
curl http://nginx.default.orb.local
```

---

## 存储配置

### 默认 StorageClass

```bash
# 查看默认存储类
kubectl get storageclass

# Orbstack 默认提供 hostpath 存储类
# NAME                 PROVISIONER
default (default)     rancher.io/local-path
```

### 创建 PVC

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF
```

---

## 镜像管理

### 本地镜像

Orbstack 与宿主机的 Docker 守护进程共享：

```bash
# 在 macOS 上构建镜像
docker build -t myapp:v1 .

# 直接在 Kubernetes 中使用（无需 push）
kubectl run myapp --image=myapp:v1 --image-pull-policy=Never
```

### 镜像仓库

```bash
# 推送到本地 registry（如果需要）
docker tag myapp:v1 localhost:5000/myapp:v1
docker push localhost:5000/myapp:v1
```

---

## 调试技巧

### 查看 Pod 日志

```bash
# 实时查看日志
kubectl logs -f deployment/myapp

# 查看之前的容器日志
kubectl logs --previous deployment/myapp

# 查看特定容器的日志
kubectl logs -f pod/myapp-pod -c my-container
```

### 进入容器调试

```bash
# 进入容器 shell
kubectl exec -it deployment/myapp -- /bin/sh

# 在容器内执行命令
kubectl exec deployment/myapp -- ls -la /app
```

### 网络调试

```bash
# 创建调试 Pod
kubectl run debug --rm -it --image=nicolaka/netshoot -- /bin/bash

# 在 debug Pod 中测试网络
# ping 其他服务
ping nginx.default.svc.cluster.local

# 测试端口连通
nc -zv my-service 80

# DNS 查询
dig my-service.default.svc.cluster.local
```

### 资源问题排查

```bash
# 查看 Pod 事件
kubectl describe pod mypod

# 查看所有事件
kubectl get events --sort-by='.lastTimestamp'

# 查看 Pod 资源使用
kubectl top pod
kubectl top node
```

---

## 常见场景

### 场景 1: 部署应用并暴露服务

```bash
# 部署应用
kubectl create deployment myapp --image=nginx

# 暴露为服务
kubectl expose deployment myapp --type=LoadBalancer --port=80

# 获取访问地址
kubectl get svc myapp
# 通过 EXTERNAL-IP 或 myapp.default.orb.local 访问
```

### 场景 2: 使用本地镜像开发

```bash
# 1. 构建镜像
docker build -t mydev:latest .

# 2. 加载到集群（如果需要）
# Orbstack 共享 Docker，通常不需要

# 3. 部署使用本地镜像
kubectl run mydev --image=mydev:latest --image-pull-policy=Never
```

### 场景 3: 清理资源

```bash
# 删除所有资源
kubectl delete all --all

# 删除特定命名空间
kubectl delete namespace mynamespace

# 清理完成 Pod
kubectl delete pods --field-selector=status.phase=Succeeded
```

---

## 故障排除

### 集群无法启动

```bash
# 检查 Orbstack 状态
orb status

# 重启 Orbstack
orb stop k8s && orb start k8s

# 重置 Kubernetes（会删除所有数据）
orb delete k8s && orb start k8s
```

### kubectl 无法连接

```bash
# 检查 kubeconfig
kubectl config view

# 重新获取 Orbstack 的 kubeconfig
# Orbstack 会自动配置，可尝试重启应用

# 手动配置上下文
kubectl config use-context orbstack
```

### DNS 解析失败

```bash
# 检查 CoreDNS 运行状态
kubectl get pods -n kube-system -l k8s-app=kube-dns

# 重启 CoreDNS
kubectl rollout restart deployment coredns -n kube-system

# 测试 DNS
dig @10.43.0.10 kubernetes.default.svc.cluster.local
```

### 镜像拉取失败

```bash
# 检查镜像是否存在
kubectl get pods -o yaml | grep -A 5 image:

# 确认 imagePullPolicy
# 本地镜像使用 Never，远程镜像使用 Always 或 IfNotPresent

# 查看详细事件
kubectl describe pod <pod-name>
```

---

## 与 Kind 对比

| 特性 | Orbstack | Kind |
|------|----------|------|
| 启动速度 | ⭐⭐⭐ 极快 | ⭐⭐ 快 |
| 资源占用 | ⭐⭐⭐ 低 | ⭐⭐ 中等 |
| 多集群 | ⭐ 单集群 | ⭐⭐⭐ 多集群 |
| 网络访问 | ⭐⭐⭐ 简单 | ⭐⭐ 需配置 |
| CI/CD 支持 | ⭐ 本地为主 | ⭐⭐⭐ 优秀 |
| 节点模拟 | ⭐ 单节点 | ⭐⭐⭐ 多节点 |

**选择建议：**
- 本地日常开发 → Orbstack
- E2E 测试、多节点测试 → Kind
- CI/CD 环境 → Kind

---

## 参考资源

- [Orbstack 官方文档](https://docs.orbstack.dev/)
- [kubectl 备忘单](https://kubernetes.io/docs/reference/kubectl/cheatsheet/)
- [Kubernetes 调试指南](https://kubernetes.io/docs/tasks/debug/debug-application/)
