这个文档描述了 Vector-Syslog-Operator 的设计和交互

## 背景

在 Kubernetes 集群中，通过 Vector 接收来自集群外部或集群内部的 Syslog / TCP/UDP Socket 日志时，通常需要手工维护：

- Vector 配置（sources/transforms/sinks）
- 对外暴露端口的 Service（LoadBalancer / NodePort）
- Vector 的运行载体（Deployment/StatefulSet）、配置热更新与回滚

当输入端口数量增多、来源团队增多时，手工维护会带来：端口冲突、配置漂移、变更不可追踪、以及上线/回滚成本上升。

本项目希望用 CRD/Controller 的方式，把"声明输入 → 自动生成配置与运行资源 → 可观测的状态反馈"固化为标准流程。

## 目的

- 以自定义资源的方式，简化 Kubernetes 中通过 Vector 接收 **裸 Socket (TCP/UDP)** 日志的流程
- （TODO）未来扩展到 Syslog（RFC3164/5424）等协议

## 范围与非目标

### 范围（当前）
- 单个 Namespace 内运行一个常驻的 Vector Aggregator（Deployment 或 StatefulSet）
- 多个 `VectorSocketSource` 描述不同的 TCP/UDP 监听入口
- 单个 `VectorSyslogConfiguration`（下文简称 Configuration）定义：
  - **全局 Pipeline**：通过 `globalPipeline.transforms` 和 `globalPipeline.sinks` 定义 transforms 和 sinks，保持 Vector 配置文件结构
  - **配置覆写**：通过 `overwriteConfig` 完全自定义任意配置段
  - Service 类型、LoadBalancerIP 与暴露策略（当前 LoadBalancer，TODO NodePort）
  - 资源与运行参数（副本数/节点调度/资源限制等）
- 允许合并额外的 ConfigMap 片段进入主配置（TODO）

### 非目标（当前不做）
- 不做任意 Vector 配置"原样透传"的通用平台
- 不支持同 Namespace 多个 Configuration 并存（避免冲突与歧义）
- 不做跨 Namespace/跨集群聚合

## 模型与约定

### Pipeline 模型

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            Pipeline Flow                                │
├─────────────────────────────────────────────────────────────────────────┤
│  1. Sources (自动生成)                                                   │
│     - 来自 VectorSocketSource CRs                                        │
│     - 命名格式: socket_<mode>_<port>                                     │
│                                                                          │
│  2. Source-specific Enrich (可选，自动)                                   │
│     - 当 VectorSocketSource 有 labels 时                                 │
│     - 命名格式: enrich_<mode>_<port>                                     │
│     - 可通过 globalPipeline.enrichEnabled: false 关闭                    │
│                                                                          │
│  3. Global Pipeline                                                      │
│     - Transforms: 用户定义在 globalPipeline.transforms 中                │
│       * key 是 transform 名称                                            │
│       * 可以使用 $$VectorSyslogOperatorSources$$ 作为 inputs             │
│     - Sinks: 用户定义在 globalPipeline.sinks 中（至少一个）              │
│       * key 是 sink 名称                                                 │
│       * 可以使用 $$VectorSyslogOperatorSources$$ 或 transform 名称       │
│                                                                          │
│  生成的配置格式为 YAML，与 Vector 原生 YAML 配置一致                     │
└─────────────────────────────────────────────────────────────────────────┘
```

### Sink 配置约定

**格式**：YAML Object，定义在 `globalPipeline.sinks` 中（必需）

**必需占位符**：`$$VectorSyslogOperatorSources$$`（有且仅有一个）

**示例**：

```yaml
spec:
  globalPipeline:
    sinks:
      elasticsearch:
        type: elasticsearch
        inputs: $$VectorSyslogOperatorSources$$  # 占位符，会被替换
        endpoints:
          - http://es:9200
        index: "logs-%Y.%m.%d"
        encoding:
          codec: json
```

**生成的配置**（YAML 格式）：

```yaml
sources:
  socket_tcp_5140:
    type: socket
    address: "0.0.0.0:5140"
    mode: tcp

transforms:
  enrich_tcp_5140:
    type: remap
    inputs:
      - socket_tcp_5140
    source: |
      .source_team = "backend"

sinks:
  elasticsearch:
    type: elasticsearch
    inputs:
      - enrich_tcp_5140
    endpoints:
      - http://es:9200
    index: "logs-%Y.%m.%d"
    encoding:
      codec: json
```

### Global Pipeline

保持 Vector 配置文件结构，支持 transforms 和 sinks：

```yaml
spec:
  globalPipeline:
    # 是否启用 source-specific enrich（默认为 true）
    enrichEnabled: true
    
    # 全局 transforms - key 是 transform 名称
    transforms:
      add_timestamp:
        type: remap
        inputs: $$VectorSyslogOperatorSources$$  # 使用占位符
        source: |
          .ingest_timestamp = now()
          .cluster = "production"
      
      filter_health:
        type: filter
        inputs: ["add_timestamp"]  # 或引用其他 transform
        condition: |
          !match(.message, r'health|/ping|/ready')
    
    # sinks - key 是 sink 名称（至少一个）
    sinks:
      elasticsearch:
        type: elasticsearch
        inputs: $$VectorSyslogOperatorSources$$
        endpoints:
          - http://es:9200
        index: "logs"
      
      console_debug:
        type: console
        inputs: ["filter_health"]  # 可以引用 transform
        encoding:
          codec: json
          json:
            pretty: true
```

**注意**：所有在 `globalPipeline.sinks` 中的配置都必须包含且仅包含一个 `$$VectorSyslogOperatorSources$$` 占位符（用于 inputs 字段）。

### Service 配置

支持 LoadBalancerIP 配置：

```yaml
spec:
  service:
    type: LoadBalancer
    # 可选：指定 LoadBalancer IP（需要底层云提供商支持）
    loadBalancerIP: "10.0.0.50"
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

### Overwrite Config

用于完全覆写或添加自定义配置段：

```yaml
spec:
  overwriteConfig:
    # key 是配置段名称，value 是 YAML 配置
    sources.internal_metrics:
      type: internal_metrics
      scrape_interval_secs: 15
    
    transforms.my_custom:
      type: remap
      inputs: ["some_source"]
      source: ".foo = 'bar'"
    
    sinks.prometheus:
      type: prometheus_exporter
      inputs: ["internal_metrics"]
      address: "0.0.0.0:9598"
```

**优先级**：
1. `overwriteConfig` 中的 `sources.*` 最先渲染
2. 自动生成的 socket sources
3. source-specific enrich transforms
4. `overwriteConfig` 中的 `transforms.*`
5. `globalPipeline.transforms`
6. `globalPipeline.sinks`
7. `overwriteConfig` 中的其他 sections

### 配置格式

**统一使用 YAML 格式**，与 Kubernetes 配置习惯一致：

```yaml
# vector.yaml (生成的配置)
sources:
  socket_tcp_5140:
    type: socket
    address: "0.0.0.0:5140"
    mode: tcp

transforms:
  enrich_tcp_5140:
    type: remap
    inputs:
      - socket_tcp_5140
    source: |
      .source_team = "backend"

sinks:
  elasticsearch:
    type: elasticsearch
    inputs:
      - enrich_tcp_5140
    endpoints:
      - http://es:9200
    index: "logs"
```

### 端口与冲突规则（必须）
- 在同一个 Configuration 选中的 Sources 集合内：`(mode, port)` 必须唯一。
- Service `ports[].name` 必须唯一（Controller 会按规则生成）。
- 若发生冲突：Controller 必须停止下发配置/资源更新，并在 Status 中给出可读的冲突信息。

### 配置生成流程

1. 读取 Configuration
2. 列出并读取匹配的 `VectorSocketSource`
3. 检查端口冲突
4. 计算 base inputs（经过 enrich 后的 source 名称列表）
5. 验证占位符（所有使用占位符的地方必须有且仅有一个）
6. 渲染 Vector 配置（YAML 格式）：
   - 构建完整的配置对象
   - 替换所有 `$$VectorSyslogOperatorSources$$` 为 base inputs
   - 序列化为 YAML
7. 创建/更新 ConfigMap
8. 创建/更新 Service、Deployment
9. 更新 Status

### 校验规则

**当前实现**：
- ✅ 端口冲突检测
- ✅ 占位符检查（globalPipeline.sinks 每个必须有且仅有一个 `$$VectorSyslogOperatorSources$$`）
- ✅ globalPipeline.transforms 占位符检查

**TODO**：
- [ ] 字段结构校验（使用 OpenAPI schema 或 Webhook）
- [ ] `vector validate` 集成（通过 Job 验证配置有效性）
- [ ] Sink 配置 schema 校验（可选，根据 type 字段）

### 状态反馈（Status）约定
- Configuration Status 至少包含：
  - `observedGeneration`
  - `selectedSources[]`（被选中的 Source 名称列表）
  - `configHash`（渲染配置的 hash）
  - `exposedPorts[]`（对外暴露端口列表）
  - `conditions[]`：如 `SourcesResolved` / `PortConflictFree` / `ConfigRendered` / `ConfigValidated` / `ResourcesApplied` / `Ready`

## CR 设计

### CR 列表

- `VectorSocketSource`：描述一个 TCP/UDP Socket 输入入口
- `VectorSyslogConfiguration`：描述本 Namespace 的 Vector 聚合器配置（单例）

### VectorSocketSource

```yaml
apiVersion: vectorsyslog.lab.camber.moe/v1alpha1
kind: VectorSocketSource
metadata:
  name: tcp-app-logs
  namespace: default
  labels:
    app: myapp
spec:
  mode: tcp              # tcp 或 udp
  port: 5140            # 监听端口
  labels:               # 注入到日志的元数据（会创建 enrich transform）
    source: app
    team: backend
```

### VectorSyslogConfiguration

```yaml
apiVersion: vectorsyslog.lab.camber.moe/v1alpha1
kind: VectorSyslogConfiguration
metadata:
  name: default
  namespace: default
spec:
  # 全局 Pipeline - 保持 Vector 配置文件结构
  globalPipeline:
    enrichEnabled: true
    
    # Transforms - key 是 transform 名称
    transforms:
      add_metadata:
        type: remap
        inputs: $$VectorSyslogOperatorSources$$
        source: |
          .cluster = "production"
          .ingest_time = now()
      
      filter_noise:
        type: filter
        inputs: ["add_metadata"]
        condition: |
          !match(.message, r'health|/ping|/ready')
    
    # Sinks - key 是 sink 名称（至少一个，必需）
    sinks:
      elasticsearch:
        type: elasticsearch
        inputs: $$VectorSyslogOperatorSources$$
        endpoints:
          - http://es:9200
        index: "logs-%Y.%m.%d"
        encoding:
          codec: json
      
      console_debug:
        type: console
        inputs: ["filter_noise"]
        encoding:
          codec: json
          json:
            pretty: true

  # 配置覆写 - 完全自定义任意配置段
  overwriteConfig:
    sources.internal_metrics:
      type: internal_metrics
      scrape_interval_secs: 15
    sinks.prometheus:
      type: prometheus_exporter
      inputs: ["internal_metrics"]
      address: "0.0.0.0:9598"

  service:
    type: LoadBalancer
    # 可选：指定 LoadBalancer IP
    loadBalancerIP: "10.0.0.50"
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"

  sourceSelector:
    matchLabels:
      app: myapp

  replicas: 2
  image: timberio/vector:latest
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi"
      cpu: "500m"
```

## Reconcile 流程（Configuration Controller）

```
1. 读取 Configuration
   └── 设置/检查 finalizer

2. 列出 Sources
   └── 根据 sourceSelector 筛选

3. 端口冲突检测
   └── 若冲突：更新 Status，停止处理

4. 计算 Base Inputs
   └── sources 经过 enrich transforms 后的名称列表

5. 验证占位符
   ├── globalPipeline.sinks 至少有一个
   ├── globalPipeline.sinks.* 每个必须有且仅有一个
   ├── globalPipeline.transforms.* 每个必须有且仅有一个
   └── 若不通过：更新 Status，停止处理

6. 渲染配置（YAML 格式）
   ├── 构建配置对象
   │   ├── sources（overwrite + 自动生成）
   │   ├── transforms（enrich + overwrite + globalPipeline）
   │   └── sinks（overwrite + globalPipeline）
   ├── 替换所有 $$VectorSyslogOperatorSources$$
   └── 序列化为 YAML

7. 创建/更新子资源
   ├── ConfigMap
   ├── Service（支持 LoadBalancerIP）
   └── Deployment

8. 更新 Status
```

## 示例文件

| 文件 | 说明 |
|------|------|
| `vectorsocketsource_tcp.yaml` | TCP/UDP Source 示例 |
| `vectorsyslogconfiguration_console.yaml` | Console Sink 基础示例 |
| `vectorsyslogconfiguration_kafka.yaml` | Kafka Sink 示例 |
| `vectorsyslogconfiguration_elasticsearch.yaml` | Elasticsearch Sink 示例 |
| `orbstack-test-25535.yaml` | Orbstack 本地测试（TCP/UDP 25535/25536） |
| `advanced-global-pipeline.yaml` | 高级用法：globalPipeline transforms + sinks + LoadBalancerIP |
