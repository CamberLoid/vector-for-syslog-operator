这个文档描述了 Vector-Syslog-Operator 的设计和交互，以及 Operator

版本 v1alpha1

## TODO

1. （v2）允许 Source 对应 Service 定义为 Nodeport

## 背景

TBF

## 目的

* 以自定义资源的方式，简化 Kubernetes 中通过 Vector 接收 Syslog/裸 Socket 日志的流程

## 模型/约定

1. 这个项目的 Pipeline 模型
   1. 多个 Socket/Syslog(TODO) Source，以及关联 transform
   2. 单个 Sink 到外部 -> 定义在 `VectorSyslogConfiguration` 内
   3. 允许用额外的 CM Merge/Overwrite 入主配置（比如自监控）
      1. 项目附带一个用于自监控的 cm，包括 internal source + prometheus exporter sink 
      2. 允许用户修改该 cm
         1. 待定：修改 CM 本身或者修改 VectorSyslogConfiguration
2. 约定
   1. （至少每个 namespace 内）只会有一个 Service （不会有两个相同service type的service）
   2. 至少每个 namespace 内只会有一个 `VectorSyslogConfiguration`
      1. Controller 独立 namespace 部署
      2. 每个 namespace 只会有一个 VectorSyslogConfiguration，以避免冲突
3. Service 类型为 LoadBalancer，定义在 VectorSyslogConfiguration 内
4. 用一个统一的 loadbalancer service 对外提供服务 -> 这样可以指定 类似 TCP 1145:1145 / UDP 4514:4514的端口
5. CM 生成流程：
   1. 读取 `VectorSyslogConfiguration` 定义的
   2. 读取额外的 CM，合并进入
   3. 对生成的项目进行 `vector validate` （TODO：这个在 Kubernetes 怎么实现）
   4. 如果 ok：更新至 deploy 对应的 CM 中，Status 反馈完成
   5. 如果 失败：Status 反馈失败


## CR 设计

本项目遵循 Kubebuilder 规范，

### Aggregator 定义 `VectorConfiguration`

`VectorConfiguration` 定义以下内容：
- Vector 镜像 `spec.image`
- 全局 transform + sink 流
  - 其中使用 `$$OperatorSource$$` （暂定）指代自动生成的 input 列表
- overwrite vector.yaml 配置
  - TODO：默认启用 internal source + prometheus sink
- 默认的 Service 定义

### 输入定义 `VectorSocketSource`

VectorSocketSource: 
- metadata 里定义 name 和 namespace
- 每个资源定义
  - 监听协议
  - 对外监听端口
  - 可选：内部监听端口
  - （可选）自定义 tag
  - Per source 的 Transform 流

TODO：VectorSyslogSource

### 



包括以下 CR/CRD：

* `VectorSocketSource`
* `VectorSyslogSource`
* `VectorConfiguration`

这些作用域仅限于单个 namespace

然后 主要 Controller 会起一个常驻 aggregator，可以是 Statefulset 也可以是 Deployment


## For Agents

1. 你需要
