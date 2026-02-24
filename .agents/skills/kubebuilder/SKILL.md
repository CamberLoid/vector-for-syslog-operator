---
name: kubebuilder
description: Kubebuilder Operator 开发框架。用于创建、开发和部署 Kubernetes Operator。当用户需要开发 Kubernetes Operator、创建 CRD、实现控制器逻辑、添加 webhook 验证、处理资源生命周期管理时使用此 skill。
---

# Kubebuilder Operator 开发

本 skill 涵盖使用 Kubebuilder 开发 Kubernetes Operator 的核心知识、最佳实践和常用模式。

---

## 目录

1. [快速开始](#快速开始)
2. [项目结构](#项目结构)
3. [核心概念](#核心概念)
4. [常用命令](#常用命令)
5. [API 设计](#api-设计)
6. [Controller 开发](#controller-开发)
7. [Webhook 开发](#webhook-开发)
8. [测试](#测试)
9. [部署](#部署)
10. [故障排除](#故障排除)

---

## 快速开始

### 初始化项目

```bash
# 初始化新项目
kubebuilder init --domain example.com --repo github.com/example/my-operator

# 创建 API 和 Controller
kubebuilder create api --group apps --version v1 --kind MyApp
```

### 项目生命周期

```
初始化项目 → 创建 API → 定义 CRD → 实现 Controller → 添加 Webhook → 测试 → 部署
```

---

## 项目结构

### 单组布局（默认）

```
├── api/
│   └── v1/                    # API 版本目录
│       ├── *_types.go         # CRD 定义（+kubebuilder 标记）
│       └── zz_generated.*     # 自动生成（禁止手动修改）
├── cmd/
│   └── main.go                # Manager 入口
├── internal/
│   ├── controller/            # 控制器逻辑
│   │   ├── *_controller.go
│   │   └── suite_test.go
│   └── webhook/               # Webhook 实现（可选）
├── config/
│   ├── crd/bases/             # 生成的 CRD（禁止手动修改）
│   ├── rbac/role.yaml         # 生成的 RBAC（禁止手动修改）
│   └── samples/               # 示例 CR
├── Makefile
└── PROJECT                    # Kubebuilder 元数据（禁止手动修改）
```

### 多组布局

当项目需要多个 API 组时使用：

```bash
kubebuilder edit --multigroup=true
```

```
api/
├── batch/v1/                  # batch 组
│   └── *_types.go
└── apps/v1/                   # apps 组
    └── *_types.go
```

---

## 核心概念

### 1. CRD (Custom Resource Definition)

自定义资源定义，扩展 Kubernetes API。

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

type MyApp struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   MyAppSpec   `json:"spec,omitempty"`
    Status MyAppStatus `json:"status,omitempty"`
}
```

### 2. Controller

控制循环：Watch → Reconcile → Update Status

```go
func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. 获取资源
    // 2. 执行业务逻辑
    // 3. 更新状态
    // 4. 返回结果（可选重试或定时重调）
}
```

### 3. Manager

管理所有控制器和 webhook 的生命周期。

### 4. Finalizer

用于资源清理，在删除前执行操作。

```go
// 添加 Finalizer
controllerutil.AddFinalizer(obj, "myapp.example.com/finalizer")

// 处理删除逻辑
if !obj.GetDeletionTimestamp().IsZero() {
    // 执行清理
    controllerutil.RemoveFinalizer(obj, "myapp.example.com/finalizer")
}
```

---

## 常用命令

### 项目管理

| 命令 | 说明 |
|------|------|
| `kubebuilder init` | 初始化新项目 |
| `kubebuilder create api` | 创建 API 和控制器 |
| `kubebuilder create webhook` | 创建 webhook |
| `kubebuilder edit` | 修改项目配置 |

### 构建与生成

| 命令 | 说明 |
|------|------|
| `make generate` | 生成 DeepCopy 方法 |
| `make manifests` | 生成 CRD 和 RBAC |
| `make lint-fix` | 自动修复代码风格 |
| `make build` | 构建二进制文件 |
| `make run` | 本地运行（使用当前 kubeconfig） |

### 测试

| 命令 | 说明 |
|------|------|
| `make test` | 运行单元测试（使用 envtest） |
| `make test-e2e` | 运行 E2E 测试 |

### 部署

| 命令 | 说明 |
|------|------|
| `make docker-build` | 构建镜像 |
| `make docker-push` | 推送镜像 |
| `make deploy` | 部署到集群 |
| `make build-installer` | 生成安装 YAML |
| `make undeploy` | 从集群卸载 |

---

## API 设计

### 常用 kubebuilder 标记

#### 资源标记

```go
// +kubebuilder:object:root=true              // 标记为根类型
// +kubebuilder:subresource:status            // 启用 status 子资源
// +kubebuilder:resource:scope=Namespaced     // 命名空间作用域
// +kubebuilder:resource:scope=Cluster        // 集群作用域
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
```

#### 字段验证标记

```go
// +kubebuilder:validation:Required           // 必填字段
// +kubebuilder:validation:Optional           // 可选字段（默认）
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=100
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=100
// +kubebuilder:validation:Pattern="^[a-z0-9-]+$"
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
// +kubebuilder:default="default-value"
```

#### 字段示例

```go
type MyAppSpec struct {
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=63
    // +kubebuilder:validation:Pattern="^[a-z0-9-]+$"
    Name string `json:"name"`
    
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=100
    // +kubebuilder:default=1
    Replicas int32 `json:"replicas,omitempty"`
    
    // +kubebuilder:validation:Enum=Small;Medium;Large
    // +kubebuilder:default=Medium
    Size string `json:"size,omitempty"`
    
    // +kubebuilder:validation:Optional
    Config map[string]string `json:"config,omitempty"`
}
```

### 状态设计

使用 `metav1.Condition` 表示状态：

```go
type MyAppStatus struct {
    // +listType=map
    // +listMapKey=type
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    Phase string `json:"phase,omitempty"`
    Ready bool   `json:"ready,omitempty"`
}

const (
    TypeAvailable = "Available"
    TypeProgressing = "Progressing"
    TypeDegraded = "Degraded"
)
```

---

## Controller 开发

### 基础模板

```go
package controller

import (
    "context"
    
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"
    
    mygroupv1 "github.com/example/my-operator/api/v1"
)

type MyAppReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mygroup.example.com,resources=myapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mygroup.example.com,resources=myapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mygroup.example.com,resources=myapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    
    // 获取 MyApp 实例
    myapp := &mygroupv1.MyApp{}
    if err := r.Get(ctx, req.NamespacedName, myapp); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // TODO: 业务逻辑
    
    return ctrl.Result{}, nil
}

func (r *MyAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&mygroupv1.MyApp{}).
        Owns(&appsv1.Deployment{}).
        Owns(&corev1.Service{}).
        Complete(r)
}
```

### 常用模式

#### 1. 创建/更新资源

```go
func (r *MyAppReconciler) reconcileDeployment(ctx context.Context, myapp *mygroupv1.MyApp) error {
    deployment := &appsv1.Deployment{}
    deployment.Name = myapp.Name
    deployment.Namespace = myapp.Namespace
    
    // 使用 CreateOrUpdate
    _, err := ctrl.CreateOrUpdate(ctx, r.Client, deployment, func() error {
        // 设置 owner reference
        if err := ctrl.SetControllerReference(myapp, deployment, r.Scheme); err != nil {
            return err
        }
        
        // 设置 spec
        deployment.Spec.Replicas = &myapp.Spec.Replicas
        deployment.Spec.Selector = &metav1.LabelSelector{
            MatchLabels: map[string]string{"app": myapp.Name},
        }
        deployment.Spec.Template = corev1.PodTemplateSpec{
            ObjectMeta: metav1.ObjectMeta{
                Labels: map[string]string{"app": myapp.Name},
            },
            Spec: corev1.PodSpec{
                Containers: []corev1.Container{{
                    Name:  "app",
                    Image: myapp.Spec.Image,
                }},
            },
        }
        return nil
    })
    
    return err
}
```

#### 2. 更新状态

```go
import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *MyAppReconciler) updateStatus(ctx context.Context, myapp *mygroupv1.MyApp, ready bool) error {
    // 重新获取以避免冲突
    latest := &mygroupv1.MyApp{}
    if err := r.Get(ctx, client.ObjectKeyFromObject(myapp), latest); err != nil {
        return err
    }
    
    condition := metav1.Condition{
        Type:    mygroupv1.TypeAvailable,
        Status:  metav1.ConditionTrue,
        Reason:  "DeploymentReady",
        Message: "Deployment is ready",
    }
    if !ready {
        condition.Status = metav1.ConditionFalse
        condition.Reason = "DeploymentNotReady"
        condition.Message = "Deployment is not ready"
    }
    
    meta.SetStatusCondition(&latest.Status.Conditions, condition)
    latest.Status.Ready = ready
    
    return r.Status().Update(ctx, latest)
}
```

#### 3. 错误处理和重试

```go
func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    
    myapp := &mygroupv1.MyApp{}
    if err := r.Get(ctx, req.NamespacedName, myapp); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 处理 Finalizer
    if myapp.DeletionTimestamp.IsZero() {
        // 对象未被删除，添加 Finalizer
        if !controllerutil.ContainsFinalizer(myapp, myFinalizer) {
            controllerutil.AddFinalizer(myapp, myFinalizer)
            if err := r.Update(ctx, myapp); err != nil {
                return ctrl.Result{}, err
            }
        }
    } else {
        // 对象正在被删除，执行清理
        if controllerutil.ContainsFinalizer(myapp, myFinalizer) {
            if err := r.cleanup(ctx, myapp); err != nil {
                return ctrl.Result{}, err
            }
            controllerutil.RemoveFinalizer(myapp, myFinalizer)
            if err := r.Update(ctx, myapp); err != nil {
                return ctrl.Result{}, err
            }
        }
        return ctrl.Result{}, nil
    }
    
    // 执行业务逻辑
    if err := r.doSomething(ctx, myapp); err != nil {
        // 错误时重试，使用指数退避
        return ctrl.Result{}, err
    }
    
    // 需要定时重调
    return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}
```

#### 4. 事件记录

```go
import (
    "k8s.io/client-go/tools/record"
)

type MyAppReconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
}

func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ...
    
    // 记录事件
    r.Recorder.Event(myapp, corev1.EventTypeNormal, "Created", "Created deployment")
    r.Recorder.Eventf(myapp, corev1.EventTypeWarning, "Failed", "Failed to create deployment: %v", err)
    
    // ...
}
```

---

## Webhook 开发

### 创建 Webhook

```bash
# 创建验证和默认值 webhook
kubebuilder create webhook --group mygroup --version v1 --kind MyApp \
  --defaulting --programmatic-validation
```

### 默认值 Webhook

```go
package v1

import (
    "sigs.k8s.io/controller-runtime/pkg/webhook"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *MyApp) SetupWebhookWithManager(mgr ctrl.Manager) error {
    return ctrl.NewWebhookManagedBy(mgr).
        For(r).
        Complete()
}

// +kubebuilder:webhook:path=/mutate-mygroup-example-com-v1-myapp,mutating=true,failurePolicy=fail,groups=mygroup.example.com,resources=myapps,verbs=create;update,versions=v1,name=mmyapp.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &MyApp{}

func (r *MyApp) Default() {
    if r.Spec.Replicas == 0 {
        r.Spec.Replicas = 1
    }
    if r.Spec.Size == "" {
        r.Spec.Size = "Medium"
    }
}
```

### 验证 Webhook

```go
// +kubebuilder:webhook:path=/validate-mygroup-example-com-v1-myapp,mutating=false,failurePolicy=fail,groups=mygroup.example.com,resources=myapps,verbs=create;update,versions=v1,name=vmyapp.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MyApp{}

func (r *MyApp) ValidateCreate() (admission.Warnings, error) {
    return nil, r.validate()
}

func (r *MyApp) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
    return nil, r.validate()
}

func (r *MyApp) ValidateDelete() (admission.Warnings, error) {
    return nil, nil
}

func (r *MyApp) validate() error {
    var allErrs field.ErrorList
    
    if r.Spec.Name == "" {
        allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("name"), "name is required"))
    }
    
    if r.Spec.Replicas < 1 || r.Spec.Replicas > 100 {
        allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("replicas"), r.Spec.Replicas, "must be between 1 and 100"))
    }
    
    if len(allErrs) > 0 {
        return apierrors.NewInvalid(schema.GroupKind{Group: "mygroup.example.com", Kind: "MyApp"}, r.Name, allErrs)
    }
    return nil
}
```

---

## 测试

### 单元测试

```go
package controller

import (
    "context"
    "testing"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
    
    mygroupv1 "github.com/example/my-operator/api/v1"
)

var _ = Describe("MyApp Controller", func() {
    Context("When reconciling a resource", func() {
        It("should create deployment", func() {
            ctx := context.Background()
            
            myapp := &mygroupv1.MyApp{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-myapp",
                    Namespace: "default",
                },
                Spec: mygroupv1.MyAppSpec{
                    Name:     "test",
                    Replicas: 3,
                    Image:    "nginx:latest",
                },
            }
            
            // 创建资源
            Expect(k8sClient.Create(ctx, myapp)).To(Succeed())
            
            // 验证 Deployment 被创建
            deployment := &appsv1.Deployment{}
            Eventually(func() error {
                return k8sClient.Get(ctx, client.ObjectKeyFromObject(myapp), deployment)
            }, timeout, interval).Should(Succeed())
            
            Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
        })
    })
})
```

### E2E 测试

```bash
# 创建 Kind 集群进行 E2E 测试
make test-e2e
```

---

## 部署

### 构建镜像

```bash
# 设置镜像标签
export IMG=myregistry.com/my-operator:v1.0.0

# 构建并推送
make docker-build docker-push IMG=$IMG

# 或使用 Kind 加载
kind load docker-image $IMG --name my-cluster
```

### 部署到集群

```bash
# 部署
make deploy IMG=$IMG

# 验证
kubectl get pods -n my-operator-system
kubectl logs -n my-operator-system deployment/my-operator-controller-manager -c manager
```

### 生成安装包

```bash
# 生成 dist/install.yaml
make build-installer IMG=$IMG

# 用户可以直接使用
kubectl apply -f https://raw.githubusercontent.com/<org>/<repo>/<tag>/dist/install.yaml
```

### Helm Chart

```bash
# 生成 Helm Chart
kubebuilder edit --plugins=helm/v2-alpha

# 重新生成（如果添加 webhook）
# 1. 备份 dist/chart/values.yaml 和 dist/chart/manager/manager.yaml
# 2. kubebuilder edit --plugins=helm/v2-alpha --force
# 3. 恢复自定义配置
```

---

## 故障排除

### 常见问题

#### 1. CRD 未更新

```bash
# 重新生成
make manifests

# 重新应用
kubectl apply -f config/crd/bases/
```

#### 2. RBAC 权限不足

```bash
# 检查生成的 RBAC
make manifests
cat config/rbac/role.yaml

# 重新部署
make deploy IMG=$IMG
```

#### 3. Webhook 证书问题

```bash
# 确保证书管理器已安装
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# 检查证书
kubectl get certificates -n my-operator-system
kubectl describe certificate -n my-operator-system
```

#### 4. Controller 不触发

- 检查 `SetupWithManager` 中的 `For` 和 `Owns`
- 验证资源 Group/Version/Kind 正确
- 查看 Controller 日志

```bash
kubectl logs -n my-operator-system deployment/my-operator-controller-manager -c manager -f
```

#### 5. 状态更新冲突

```go
// 解决方案：重新获取最新版本
latest := &mygroupv1.MyApp{}
if err := r.Get(ctx, client.ObjectKeyFromObject(myapp), latest); err != nil {
    return err
}
// 修改 latest 的状态
return r.Status().Update(ctx, latest)
```

---

## 最佳实践

### 1. 幂等性

Reconcile 方法应该可以安全地多次执行。

### 2. Owner Reference

使用 `SetControllerReference` 启用垃圾回收。

### 3. Watch 相关资源

使用 `.Owns()` 或 `.Watches()` 而不是 `RequeueAfter`。

### 4. 结构化日志

```go
log := log.FromContext(ctx)
log.Info("Processing", "name", obj.Name, "namespace", obj.Namespace)
log.Error(err, "Failed to create deployment", "deployment", deployment.Name)
```

### 5. 资源命名

避免在资源名称中包含随机字符，使用确定性的命名。

### 6. 验证标记

尽可能使用 kubebuilder 验证标记，减少 webhook 代码。

---

## 参考资源

- [Kubebuilder Book](https://book.kubebuilder.io)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
- [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
