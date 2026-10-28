<p align="center">
  <img src="docs/screenshot.png" width="900" alt="BaudScan 界面">
</p>
<p align="center"><sub>浅色主题 · <a href="docs/screenshot-dark.png">深色主题</a> · <a href="docs/screenshot-unsupported.png">驱动不支持某档时的提示</a></sub></p>

# 串口波特率扫描（BaudScan）

接入一个未知波特率的串口设备，点击一次即可自动找出最可能的波特率。工具只读取串口，**不会向设备发送任何字节**。

使用 Go + [mygo](https://github.com/egoist/mygo) 原生 UI 编写，单个二进制，支持 macOS / Windows / Linux，无需 WebView 或前端构建。

## 功能

- 自动枚举串口，选择设备后一键扫描
- 全量测试 37档标准波特率（50 – 4000000），无需选择范围
- 表格一开始就列出全部档位，扫描逐行填充，待测项排在最后
- 每档采样 0.4 / 0.6 / 1.0 / 2.0 / 5.0 秒可选，界面显示全扫预计耗时
- 驱动拒绝的高速率标为“不支持”，不计入错误
- 实时显示扫描进度，可随时停止
- 按可信度排序候选，自动选中最佳结果
- 展示可读字符比例、文本行数、重复结构、字节熵
- 十六进制 + ASCII 原始样本，可选中复制
- 只读扫描；打开一次串口后仅切换波特率，避免反复开关触发开发板复位
- 纯 Go，无 cgo；可交叉编译到三大平台

## 识别原理

对每个候选波特率：

1. 以 8N1 打开串口（整个扫描只打开一次），等待设备稳定，清空输入缓冲。
2. 切换到该波特率，读取一段数据；设备有输出后空闲则提前结束，否则等满采样时长。
3. 计算四类特征：
   - **可读字符比例**：正确的文本波特率会得到完整的 ASCII / UTF-8 文本；错误波特率接近随机字节。
   - **文本行结构**：CR/LF、可打印行、行长度。
   - **重复结构**：固定帧头、行周期等自相似性；可识别部分二进制协议。
   - **字节熵**：排除均匀随机噪声和恒定电平。
4. 综合评分并排序；前两名分数接近时标记“存疑”，提示对比原始样本。

扫描过程中不写串口，也不发送任何探测字符。串口打开本身可能拉高 DTR，部分开发板会因此复位一次；工具只在开始时打开一次，后续仅调用 `SetMode` 改波特率，避免每档都复位。

## 使用

1. 用 USB 转串口线或开发板接入电脑。
2. 打开 BaudScan，选择串口，点击“刷新”可重新扫描端口。
3.选择每档采样时间，点击“开始扫描”（默认 1.0 秒，全扫约 39 秒）。
4.等待结果。全部 37 档都会测到，最佳候选显示在右侧，包含评分、原始字节和判断依据。

建议：

-设备周期输出很慢时，把每档采样调到 2.0 或 5.0 秒；全扫耗时会相应变成约 78 秒或 195 秒。
- 前两名接近时，点击两行分别查看十六进制样本，选择文本/帧结构更自然的一个。
- Linux 没有串口权限时，把用户加入 `dialout` 组后重新登录：
  ```sh
  sudo usermod -aG dialout "$USER"
  ```
- macOS 上同时出现蓝牙端口时，优先选择 `usbmodem` / `usbserial` 端口。

## 支持范围

默认只按 **8N1（8 数据位、无校验、1 停止位）** 尝试波特率，这也是绝大多数设备的选择。工具不检测校验位、停止位或数据位组合。

### 测试的 37 档波特率

`50,75,110,134,150,200,300,600,1200,1800,2400,4800,7200,9600,14400,19200,28800,38400,57600,76800,115200,128000,230400,250000,256000,460800,500000,576000,921600,1000000,1152000,1500000,2000000,2500000,3000000,3500000,4000000`

扫描顺序先走常用速率（115200、9600、57600、38400、19200、230400、921600、4800、2400、1200），再补齐其余档位，因此常见设备的结果会先出现。最终表格按评分排序。

Linux用 `BOTHER`、macOS用 `IOSSIOSPEED`、Windows用 DCB设置非标准速率，所以37档在所有平台都会被尝试。macOS的部分串口驱动会拒绝230400以上速率，这些档位标为“不支持”，不影响其它档位。

### 无法可靠识别的场景

- 设备只在收到命令后才回复：工具不发送任何数据，因此收不到样本。
- 输出是加密/压缩的随机二进制：没有可读文本或周期结构可供判断。
- 极低占空比的一次性开机消息：采样窗口可能错过；可增大采样时间并重新扫描。
- 使用非 8N1 格式的设备：可能完全读不到有效数据。

## 开发

要求 Go 1.27.3 或更新（`go.mod` 会通过 `GOTOOLCHAIN=auto` 自动下载所需版本）。原生 UI 不需要 Bun。

```sh
# 开发模式（生成并启动开发版 app，修改 .go 后自动重启）
go tool mygo dev

# 测试
go test ./...
go vet ./...

# 打包当前平台（macOS 生成 .app + .dmg）
go tool mygo build

# macOS 启动打包后的应用（Apple Silicon 把 amd64 换成 arm64）
open build/darwin-amd64/BaudScan.app
```

macOS 上请通过 mygo 生成的 `.app` 运行：直接 `go run .` 或裸二进制不会创建原生窗口。Windows / Linux 运行 `build/<os>-<arch>/` 下的可执行文件。

mygo 构建产物在 `build/`。

### 一键构建 x64 发布包

在 macOS 上执行：

```sh
./build.sh
```

脚本会构建全部三个 x64 目标，并把最终文件放进 `dist/`：

| 文件 | 平台 | 说明 |
|---|---|---|
| `BaudScan-<version>-macos-x64.dmg` | macOS Intel | 已校验的磁盘镜像 |
| `BaudScan-<version>-windows-x64.exe` | Windows x64 | GUI 子系统 PE，无控制台窗口 |
| `BaudScan-<version>-linux-x64` | Linux x64 | 可执行 ELF 二进制 |
| `SHA256SUMS` | — | 三个产物的 SHA-256 |

DMG 依赖 macOS 的 `hdiutil`，因此这个脚本需在 macOS 上运行。Windows 安装器需要 NSIS，脚本不生成安装器，只生成可直接运行的 exe。中间构建目录为 `build/release/`，最终产物目录 `dist/` 已加入 `.gitignore`。

### 交叉编译

如果只需要可执行文件，项目不依赖 cgo，可手动交叉编译：

```sh
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o dist/baudscan-darwin-arm64 .
CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o dist/baudscan-darwin-amd64 .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o dist/baudscan-windows-amd64.exe .
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o dist/baudscan-linux-amd64 .
```

运行环境：

- macOS 12+（原生 UI 使用 Metal）
- Windows 10/11 x64（原生 UI 使用 Direct3D 11）
- Linux x64/arm64（原生 UI 需要 GTK 3；打包时用 `go tool mygo build` 生成桌面入口和 `.deb`）

### 目录结构

```
main.go                 窗口入口
build.sh                一键构建 x64 发布包
app.go                  状态、扫描编排、端口刷新
view.go                 mygo 原生 UI
internal/baud/          采样、评分、报告格式化
internal/ports/         纯 Go 串口枚举（darwin/linux/windows）
docs/screenshot.png    界面截图
mygo.json               应用名称、标识、版本
```

## 测试

- 评分单元测试：文本、UTF-8、噪声、恒定电平、二进制周期帧。
- 扫描集成测试：用按波特率提供不同数据的模拟串口，验证能从噪声中选出 9600。
- 覆盖取消、无数据、SetMode 失败、端口刷新、结果选择、深色模式与最小窗口布局。
- `go test -race ./...`。

可选的真实串口 I/O 测试（macOS/Linux，需要 `socat`）：

```sh
socat pty,raw,echo=0,link=/tmp/baudscan-pty-a pty,raw,echo=0,link=/tmp/baudscan-pty-b
BAUDSCAN_PTY=/tmp/baudscan-pty-a BAUDSCAN_PTY_PEER=/tmp/baudscan-pty-b \
  go test ./internal/baud -run TestLivePTY
```

它验证真实的 `serial.Open`、`SetMode`、清缓冲和读取路径；PTY 会原样传递字节，所以不能验证波特率判别本身。

> 自动识别是启发式判断，不保证 100% 正确。最终结果请结合原始字节和设备协议确认。
