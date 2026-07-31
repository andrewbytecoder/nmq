---
title: "Plugin: dpcore"
description: "NCP 的核心数据与部署插件，负责 SQLite、部署器、Helm client 和微平台 client。"
---

# Plugin `dpcore`

代码入口：`plugins/dpcore/dp_component.go`

`dpcore` 是 `dpproxy` 的后端能力中心。它不直接提供 Web 路由，但几乎所有核心业务能力都从这里暴露出来。

## 核心职责

### 1. 创建 SQLite 存储

`Init()` 中首先创建：

```go
storage.NewStorage(...)
```

并立即执行：

- `storage.AutoMigrate(dp.storage)`

`plugins/dpcore/storage/storage.go` 显示它当前重点支持：

- SQLite
- 表级读取 `ReadSQLiteTable(...)`
- 多类 repository 的惰性加载

### 2. 暴露统一存储能力

`GetInterface(...)` 里它会暴露一整组数据能力：

- `dp-storage`
- `dp-idcinfo-storage`
- `dp-productinfo-storage`
- `dp-deployipinfo-storage`
- `dp-certinfo-storage`
- `dp-topoinfo-manager`
- `dp-operate-log-storage`
- `dp-servicegroupmng-storage`
- `dp-configdata-storage`

这些能力后来都被 `dpproxy` 统一取走并装配到 API 层。

### 3. 创建 Helm client

`Init()` 中使用：

- `plugins/dpcore/helmclient`
- 默认版本 `helm/v3`

`helmclient/helm.go` 展示了它统一封装了：

- `Install`
- `Upgrade`
- `Uninstall`
- `Get`
- `Template`

也就是 NCP 的 Helm 能力并不直接散落在业务代码里，而是先被这个插件封装为可调用能力。

### 4. 创建微平台 client

`Init()` 中还会创建：

- `microclient.NewMicroClient(...)`

它依赖：

- 当前节点地址
- 微平台 HTTPS 端口
- timeout
- https scheme

说明 `dpcore` 还负责与外部微平台 Service Manager 的对接。

### 5. 创建部署器

`deploy.NewDeployer(...)` 会把以下依赖装进部署器：

- NCP context
- 产品版本存储
- 拓扑存储
- 部署信息存储
- 操作日志存储
- Helm client
- Micro client

`plugins/dpcore/deploy/deploy.go` 可以看到部署器内部至少承担：

- 包处理任务
- 端口同步
- 部署任务队列
- 产品部署
- 升级 / 回滚
- 解压与版本解析
- remote 主机操作

### 6. 启动时同步关键状态

`Start()` 中它会执行：

- `dp.storage.AutoMigrate()`
- `dp.deploy.SyncPortInfo()`

这意味着这个插件在运行启动阶段会确保：

- 数据结构就绪
- 端口缓存/分配器状态同步

## Helm chart 渲染

你给出的这一段链路，核心落点在：

- `plugins/dpcore/deploy/chart_values.go`
- `manifest/dpproxy/packages/IMS-36/DPConfig/charts`
- `manifest/dpproxy/packages/IMS-36/DPConfig/manifest`

这部分功能可以理解为：`dpcore` 先按运行时、拓扑、产品详情和端口分配结果组装 `values.yaml` 数据，再交给 Helm 渲染成最终 manifest。

### 1. chart 模板来源

默认 chart 模板目录在：

```text
manifest/dpproxy/packages/IMS-36/DPConfig/charts
```

这个目录下是按服务拆分的 chart 子目录，例如：

- `acs`
- `cp`
- `etcd`
- `redis`
- `sipgw`
- `wpagent`

在实际部署时，`dpcore` 不会固定写死某一个 chart，而是根据服务名从部署详情中找到对应的 `templateDir`，再拼出当前服务的 chart 路径。

### 2. chart 路径如何确定

`plugins/dpcore/deploy/deploy_product.go` 里的 `ConstructProductDeployTopoInfoByPodIDs(...)` 会为每个待部署 Pod 计算：

- `chartPath`
- `manifestPath`
- `remoterManifestPath`
- `releaseName`

其中：

- `chartPath = <packages>/<product-id>/DPConfig/charts/<templateDir>`
- `manifestPath = <packages>/<product-id>/DPConfig/manifest/<templateDir>`
- `releaseName = <serviceName>-<instanceID>`

这一步把“某个 Pod 对应哪个 chart 模板、最终渲染结果落到哪里”固定下来。

### 3. values 组装入口

values 的总入口是：

```go
func (d *Deployer) constructCharValues(dc *deployCtx, podTopo PodTopo) (*ValuesConfig, error)
```

它按运行环境分流：

- `RuntimeEnvK8s` -> `constructKubeChartValues(...)`
- `RuntimeEnvDocker` -> `constructMinimalChartValues(...)`

也就是说，同一套 chart 渲染逻辑在 K8s 和小型化 / Docker 场景下会组装出不同的 values 数据结构。

### 4. K8s 场景 values 组装内容

`constructKubeChartValues(...)` 主要注入：

- 产品名 `YspProductName`
- 外部 K8s 类型 `ExternalK8SType`
- 是否外接 K8s
- 是否使用 PVC
- 网络模式 `NetworkMode`
- Pod ID
- Service 名
- 副本数 `Replicas`
- 主容器镜像仓库、Tag、PullPolicy
- Sidecar 镜像仓库、Tag、PullPolicy
- NodeSelector

它的数据来源主要是：

- `deployCtx.productDetailInfoMap`
- `deployCtx.serviceVersionInfoMap`
- `masterTopo`

### 5. Docker / 小型化场景 values 组装内容

`constructMinimalChartValues(...)` 更完整，除了上面的通用信息，还会注入：

- `ProductID`
- `Namespace`
- `YspProductType`
- `ProductVersion`
- `NodeName`
- `DpHost`
- `DpPort`
- `PriorityClass`
- `InternalIP`
- `ExternalIP`
- CPU / Memory requests 与 limits
- ServiceID
- 端口信息
- 主业务镜像路径 `ImagePath`
- 平台级镜像路径 `Plat.ImagePath`

这说明小型化场景下，`dpcore` 会把更多运行时细节直接烘进 values，而不只是简单传几组镜像名。

### 6. 端口是如何注入 values 的

`constructPortValues(...)` 会分三轮把端口写进 values：

1. 先注入平台级端口
2. 再注入当前产品级端口
3. 最后用当前 Pod/Service 自己的端口覆盖同名项

这里依赖的是 `deployInfoStorage` 和 `PortAllocator` 的结果，所以 chart 渲染并不是静态模板替换，而是包含部署态资源分配结果的。

### 7. 镜像路径如何注入 values

`getImagePath(...)` 会：

1. 读取当前 NCP 地址
2. 查找 `GOFASTDFS_PORT`
3. 根据 `product_name + file_name + fileType + version` 查询部署文件信息
4. 最终拼出一个可下载镜像或包文件的 URL

因此 values 里的镜像路径不是固定字符串，而是和部署文件库、产品类型、执行类型、版本信息联动生成的。

### 8. manifest 最终落盘位置

chart 渲染后最终输出目录是：

```text
manifest/dpproxy/packages/IMS-36/DPConfig/manifest
```

在实际按产品实例部署时，代码里更准确的落盘位置是：

```text
<packages>/<product-id>/DPConfig/manifest/<templateDir>/<releaseName>.yaml
```

所以 `IMS-36/DPConfig/manifest` 可以看作默认结构模板，而真正部署时会在对应产品实例目录下生成最终 YAML。

## 部署过程

这一部分主要落在：

- `plugins/dpcore/deploy/deploy_product.go`
- `plugins/dpcore/deploy/deploy.go`

`dpcore` 在这里做的事情不只是“调用 helm install”，而是把产品目录、拓扑、端口、manifest、远端同步和 apply 全部串成一个完整流程。

### 1. 获取部署任务和产品上下文

`DeployProduct(productId, sessionId)` 开始时会：

- 从任务队列中取出当前任务
- 获取 YSP 平台产品信息
- 获取当前待部署产品信息
- 获取产品版本信息

这里的产品信息来自：

- `productStorage`
- `topoInfoStorage`
- `deployInfoStorage`

### 2. 复制产品目录到实例目录

代码会把：

```text
<packages>/<product-type>-<product-version>
```

复制成：

```text
<packages>/<product-type>-<product-id>
```

也就是把“版本模板目录”复制成“本次实例部署目录”。后续所有 chart 渲染和 manifest 输出，都在这个实例目录里进行。

### 3. 同步产品目录到远端 master 节点

`DeployProduct(...)` 会从任务里取出所有 master 类型 remoter，然后把本地实例目录上传到：

```text
<remoterWorkDir>/<pkgDir>/<product-id-dir>
```

这说明部署过程天生支持“本地准备部署资产，再同步到远端主节点执行”。

### 4. 解析部署详情和拓扑

接下来会加载：

- `deploy_details.yaml`
- 产品 topo 信息
- master topo 信息

并构造 `deployCtx`，其中包含：

- `productDetailInfoMap`
- `serviceVersionInfoMap`
- `topo`
- `masterTopo`
- `apply.IApply`

这些上下文后续贯穿整个渲染和部署流程。

### 5. 生成待部署 Pod 队列

`ConstructProductDeployTopoInfo(...)` 会：

1. 遍历 topo，只挑 `NodeTypePod`
2. 根据服务名从 detail 里找对应模板目录
3. 为每个 Pod 构建 `PodTopo`
4. 给每个 Pod 赋 deployment order
5. 再把 Core/Base 类型 service 绑定回对应 Pod
6. 最后压入优先队列

因此部署顺序不是随机的，而是由部署详情中的 `order` 控制。

### 6. 逐个服务执行部署

`DeployProduct(...)` 最终会循环：

```go
for !pt.Pods.Empty() {
    podTopo := pt.Pods.Pop()
    err := d.installService(&dc, podTopo)
}
```

当前实际走的是：

- `installService(...)`
- `installOneService(...)`

也就是“逐个 Pod / Service 部署”的模型。

### 7. 单服务部署步骤

`installOneService(...)` 的关键步骤是：

1. `allocPorts(...)`
2. `constructCharValues(...)`
3. `yaml.Marshal(v)`
4. `helmClient.Install(chartPath, releaseName, values, helm.UpgradeOptions{Namespace: ...})`
5. 把 `releaseInfo.Manifest` 保存到本地 manifest 目录
6. 把生成的 YAML 上传到远端 master
7. 调用 `apply.Apply(...)` 执行真正的部署

这可以总结成：

```text
分配端口 -> 组装 values -> Helm 渲染 -> 保存 manifest -> 上传 manifest -> apply 到目标环境
```

### 8. Helm 在这里扮演什么角色

从代码看，`helmClient.Install(...)` 的结果不只是“直接在集群里安装”，更重要的是拿到：

- `releaseInfo.Manifest`

然后 `dpcore` 会把这个 manifest 作为明确的中间产物保存下来。因此 Helm 在这里既是安装器，也是渲染器。

### 9. 为什么有 manifest 目录

因为 NCP 不是只做一次性 `helm install`，它还要：

- 保存渲染结果
- 远端分发
- 做 apply
- 便于排查

所以：

- `charts/` 是模板输入
- `manifest/` 是渲染输出

这个边界在 `dpcore` 插件里是非常明确的。

## 从目录结构看它的内部模块

`plugins/dpcore` 包含：

- `storage`
- `deploy`
- `helmclient`
- `microclient`
- `fastfdclient`

其中最核心的是前四个。

## 最重要的能力总结

可以把 `dpcore` 看成 NCP 的：

- 数据平面后端
- 部署执行内核
- Helm 适配层
- 微平台对接层

如果 `dpproxy` 是“对外说话的人”，那 `dpcore` 就是“真正做事的人”。
