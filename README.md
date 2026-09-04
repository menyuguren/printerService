# 本地打印机转网络打印机

这是一个 Go 实现的基础网络打印代理。它提供两个端口：

- Web 控制端口：默认 `127.0.0.1:8080`
- 打印机数据端口：默认 `0.0.0.0:9100`

未指定打印机时，服务使用 Windows 系统默认打印机。用户可以通过 Web 控制界面指定打印机，也可以清除指定配置恢复默认打印机。

## 运行

```powershell
go run ./cmd/printerService
```

打开：

```text
http://127.0.0.1:8080
```

配置文件默认保存到：

```text
%ProgramData%\printerService\config.json
```

也可以指定配置文件：

```powershell
go run ./cmd/printerService -config .\config.json
```

## 打印方式

其他电脑需要按 RAW TCP 网络打印方式连接服务端主机的数据端口，例如：

```text
服务端IP:9100
```

客户端仍需要使用与目标打印机兼容的打印驱动。本服务不直接解析 Word、PDF、图片文件内容，只接收客户端打印驱动生成后的打印数据。

## 构建

```powershell
go build -o printerService.exe ./cmd/printerService
```
