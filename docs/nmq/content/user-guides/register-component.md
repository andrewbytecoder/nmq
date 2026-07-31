---
title: "Register a Component"
description: "按当前仓库方式为 NCP 增加一个新组件。"
---

# Register a Component

这份指引适合你要把一段新能力做成 NCP 的正式组件。

## 第一步：实现组件接口

最小骨架如下：

```go
type MyComponent struct {
    ncp.ComponentBase
}

func NewMyComponent(ctx ncp.Context) *MyComponent {
    return &MyComponent{
        ComponentBase: ncp.NewComponentBase(ctx),
    }
}
```

然后实现：

- `Init() error`
- `Start() error`
- `Stop() error`
- `Reset() error`
- `GetName() string`
- `GetVersion() string`
- `GetInterface(uuid string) any`

## 第二步：定义 capability 名

如果要给别的组件调用，请在 `interfaces/componet_name.go` 增加新的能力名常量。

## 第三步：注册组件

在 `cmd/ncp/ncp.go` 的 `RegisterComponents(...)` 中追加：

```go
ncp.RegisterComponent("my-component", mycomponent.NewMyComponent(ncp))
```

## 第四步：通过接口而不是直接依赖

其他组件获取你的能力时，应该走：

```go
cap := ctx.GetInterface("my-capability")
```

这样才能维持当前仓库强调的组件隔离。
