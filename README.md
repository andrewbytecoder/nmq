# nmq
Not only a message queue






## 代码生成


### deepcopy-gen

安装代码生成工具deepcopy-gen

```bash
go install k8s.io/code-generator/cmd/deepcopy-gen@latest
```
对需要生成辅助代码的结构体上添加标注

```bash
// +k8s:deepcopy-gen=true
// MyConfig is the root configuration.
type MyConfig struct {
    Name    string
    Servers []Server
    Options map[string]string
}

// +k8s:deepcopy-gen=true
type Server struct {
    URL    string
    Weight *int
}

```

执行代码生成指令
```bash
deepcopy-gen --output-file zz_generated.deepcopy.go ./internal/config/dynamic/...


```
或者根目录中添加generate.go文件，并把以下内容添加进去
```bash
//go:generate deepcopy-gen --output-file zz_generated.deepcopy.go ./internal/config/dynamic/...

package nmq

```


