# BaudScan ·项目速查（供编码代理）

>**基线**：通读全部源码后生成，`main` @ `fcd6365`，v0.1.0（2026-10-28）。
>**规则**：代码改了请同步改本文件；它必须描述现状，不是愿望。冲突时以代码为准。

## 1.一眼看懂

把未知波特率的串口设备接到电脑，点击一次即可扫描37档标准速率（可另加自定义速率），按统计评分给出最可能的
波特率和原始字节。**只读**：代码里不存在向串口 `Write`的路径。Go + [mygo](https://github.com/egoist/mygo)
原生 UI（无 WebView、无 cgo），三平台交叉编译。

## 2.技术栈

|项 |值 |
| --- | --- |
| module | `github.com/j4ckzh0u/BaudScan` |
| Go |1.27.3（`go.mod`的 `tool`行引入 mygo；`GOTOOLCHAIN=auto`） |
| UI | `github.com/egoist/mygo v0.7.1`，命令 `go tool mygo` |
|串口 | `go.bug.st/serial v1.8.0`（读写 +速率设置） |
|其他 | `golang.org/x/sys v0.48.0`（Windows注册表枚举） |
|许可 | MIT，©2026 j4ckzh0u |

## 3.文件地图

|文件 |行数 |职责 |
| --- | --- | --- |
| `main.go` |29 | mygo入口：`WhenReady`建窗口1120x760（最小980x680，`StateKey:"main"`）；`OnClose`调 `stopScan` |
| `app.go` |418 |状态与编排：端口刷新、开始/停止/收尾、结果入库与排序、选中跟随、状态分级 |
| `view.go` |501 |声明式 UI：header / controls /结果表 /详情面板 / footer |
| `internal/baud/scan.go` |366 |速率表与扫描顺序、`Scan`、`Rank`、`readWindow`、错误映射 |
| `internal/baud/analyze.go` |270 | `Analyze`：文本 /二进制双假设评分与 `Kind`判定 |
| `internal/baud/format.go` |151 | `Preview`（文本转义或十六进制）、`HexDump`、`FormatBytes`、`KindLabel` |
| `internal/baud/frame.go` |181 | `Frame`、标签与解析（`ParseRates` / `Plan` / `CustomRates`） |
| `internal/ports/` |76+33+54+42+9 |纯 Go串口枚举（不用 cgo版 enumerator），按 build tag分平台 |
| `build.sh` | — | x64三平台发布、校验、`dist/SHA256SUMS` |
| `mygo.json` | — |应用元数据；**版本号唯一来源** |
| `design/icon.svg` → `resources/icon.png` | — |图标源与1024x1024位图，mygo生成 .icns/.ico |
| `docs/screenshot*.png` | — | README截图（由测试环境变量导出） |

## 4.运行时主线

```text
main
 newApp()
 baud.NewScanner() // Open = serial.Open，测试可注入 fake
 listPorts = ports.List //测试替换成假函数
 resetResults() //按 a.plan预置行数（默认37），全为 Pending，表格先显示全量
 refreshPorts(false) //保留用户选择；端口消失则清空结果

startScan() -> go scanner.Scan(ctx, ScanOptions{Port, Rates: a.plan, Frame, Sample, OnEvent})
 serial.Open(frame, rates[0]) //只开一次；换档仅 SetMode，不碰 DTR
 sleep(OpenDelay220ms); ResetInputBuffer()
 for each rate:
 SetMode(rate) --fail--> Result{Err, Unsupported}并继续
 sleep(Settle45ms); ResetInputBuffer() //丢弃上一档残留
 emit(in-progress) -> readWindow -> Analyze -> Result -> emit(done)
 OnEvent在扫描 goroutine执行，经 a.update(win, fn)切到 UI线程

store(res) //按 Baud值定位行写入，再 baud.Rank(a.results)整表重排
selectBest() //首个 !Pending && Bytes>0的行；manualPick时不动
finishScan(err) //状态文案 +颜色分级（good/warn/bad）
```

`Scan`返回契约：取消 → `context.Canceled`；读错误 →已采结果 + error；
`SetMode`失败**不**中断扫描，只记 `Unsupported`或 `Err`行。

## 5.硬约束（改代码前必读）

1. **绝不写串口**：无 `port.Write`；README、UI都声明只读。新功能不得引入写路径。
2. **只 `Open`一次**：反复开关会经 DTR复位开发板；换档只能 `SetMode` +清输入缓冲。
3. **表格恒为 `len(a.plan)`行**：`store`用 `Baud`值匹配行；`Rank`会改行序，
 `selected`是**行索引**，重排后由 `selectBest`重定位；`ui.ListState.Key`用 `Baud`，
让选中与滚动跟随速率而非行号。
4. **`Pending`行排最后**；同分按 `a.plan`的扫描顺序（常用速率优先）。
5. **驱动拒绝 ≠失败**：`syscall.EINVAL`或端口拒绝速率/帧格式（InvalidSpeed / InvalidDataBits / InvalidParity / InvalidStopBits）→
 `Unsupported=true`，显示「不支持」，继续扫。
6. **版本号只读 `mygo.json`**；`build.sh`读不到立即失败。
7. **帧格式改动清空结果**：样本属于旧帧格式，`frameChanged()`会`resetResults()`。
8. UI文案全中文、状态行用 `·`分隔；测试按精确子串断言，改文案必须同步测试。

## 6.关键常量

|名称 |值 |位置 |
| --- | --- | --- |
| `DefaultSample` |900 ms（app默认选1.0秒档） | scan.go |
| `DefaultSettle` |45 ms | scan.go |
| `OpenDelay` |220 ms | scan.go |
| `IdleAfter` |140 ms（默认值在 `withDefaults`） | scan.go |
| `MinSample` |320 ms | scan.go |
| `maxCapture` |256 KiB | scan.go |
| `previewBytes` |4096（`Result.Data`保留量） | scan.go |
|读超时 |40 ms | scan.go `SetReadTimeout` |
|详情十六进制 |前192字节，每行8字节 | view.go + `HexDump` |
|表格样本预览 |100字符 | view.go |
|采样档位 |0.4 /0.6 /1.0 /2.0 /5.0秒 | app.go `durationOptions` |
|全扫估算 | `37*(Sample+Settle)+OpenDelay` | app.go `estimate` |

## 7.核心类型（签名速查）

```go
type Result struct {
 Baud int
 Bytes int
 Data []byte //前4096字节，供预览
 Analysis Analysis
 Err string
 Unsupported bool
 Pending bool
}
type Event struct { Index, Total, Baud int; Result *Result } // Result==nil表示开始测该档
type ScanOptions struct {
 Port string
 Rates []int
 Sample time.Duration
 IdleAfter time.Duration
 MinSample time.Duration
 Settle time.Duration
 OnEvent func(Event)
}
type Scanner struct { Open func(name string, mode *serial.Mode) (serial.Port, error) }

type Analysis struct {
 Bytes int
 TextRatio float64
 PrintableRatio float64
 ControlRatio float64
 Entropy float64
 Distinct int
 Lines int
 GoodLines int
 LineRatio float64
 Repeat float64
 RepeatLag int
 TextScore float64
 BinaryScore float64
 Score int
 Kind Kind
}

app{ win, scanner, listPorts, ports, port, duration, scanning, cancel, scanStarted,
 progress, status, statusLevel, previewFor, previewText,
 results []baud.Result, selected int, manualPick bool, table ui.ListState }
```

## 8.速率表

- `Standard`：37档升序，50 …4000000（scan.go:21）。
- `Common`：`115200,9600,57600,38400,19200,230400,921600,4800,2400,1200`。
- `All = scanOrder(Standard, Common)`：常用速率前移，其余升序补齐；`len(All)==len(Standard)==37`（默认计划）。
- `Plan(extra)`：Common →自定义 →其余标准升序；界面用 `a.plan`，不再直接用 `All`。
- `Rank(results, order)`：Pending最后 →分数降序 → `order`（即 `a.plan`）顺序 →兜底 `Baud`降序。

## 9.评分算法（analyze.go）

统计量：可读字节占比、控制字节占比、字节熵、不同字节数、行的数量与「好行」比例、
周期性 `Repeat`（滞后2…min(len/3,384)的最高字节重合率，只取前8192字节）。

```text
text = clamp01((TextRatio -0.42) /0.53)
line = LineRatio * {0行:0,1行:0.45,2行:0.75,其它:1}
repeat = clamp01((Repeat -0.01) /0.16)
entropy = clamp01((7.75 - Entropy) /2.6)
clean =1 - clamp01(ControlRatio /0.18)
distinct = clamp01((Distinct -1) /8)

TextScore =100*(0.55*text +0.18*line +0.10*repeat +0.10*entropy +0.07*clean) * (0.72 +0.28*distinct)
binClean =1 - clamp01(ControlRatio /0.5)
BinaryScore =100*(0.68*repeat +0.14*entropy +0.12*binClean +0.06*distinct)
Score = round(max(TextScore, BinaryScore))
```

判定顺序：空 → `no-data`；`len>=6 && Distinct<=2` → `constant`/0；短样本封顶
（`<2`:0、`<4`:30、`<8`:50、`<16`:70）；`Score<20 || (text<0.25 && BinaryScore<25)` → `noise`；
`TextScore>=BinaryScore && text>=0.25` → `text`；`BinaryScore>=40` → `binary`；否则 `noise`。

界面阈值：`>=75`高置信度、`>=50`中等、其余低；`finishScan`在 `best.Score>=60`报「完成·最可能」；
`closeRace` =前两名均非 Pending且 `best>=30 &&差<=6` → 「存疑」并降为 warn；
`row==0 && Score>=50`显示「·最佳」。

## 10. UI结构（view.go）

```text
view()
 Column(padding22, gap14)
 header title + subtitle + pill("只读模式 ·不发送数据")
 controls row1: port Select(值=端口名, Grow) + refresh |数据位 |校验位 |停止位(各140) |自定义波特率(178)
 controls row2:左列 =提示行 +状态点/文案 +计划行;右:每档采样 Select(150) |开始扫描 |停止 (+ Progress)
 Row(grow1)
 results card Table: baud92(end) | score58(end) | bytes74(end) | kind118 | sample >=150
 detail card width404: score/confidence pill,4 metrics, TextArea(hex dump192B), verdict
 footer read-only notice + rate count + estimate
```

详情分支优先级：`selected<0` → emptyDetail；`Pending` → pendingDetail；`Unsupported` → 「设备不支持」；
`Err!=""` → 「无法读取」；否则完整指标。`previewText`以 `port/baud/bytes`为 key决定是否重建，
避免每帧覆盖用户选中文本。点表格任一行 → `manualPick=true`，直到下次扫描。

## 11.平台差异

|平台 |枚举 |速率设置 |运行要求 |
| --- | --- | --- | --- |
| macOS | `filepath.Glob("/dev/cu.*")`（不用 tty.*，会等载波）；按名字标蓝牙 / USB /调试控制台 | `IOSSIOSPEED` ioctl，取决于驱动 |12+，Metal；**必须经 `.app`启动才有窗口**，`go run .`无窗口 |
| Linux |11个 glob：ttyUSB / ttyACM / ttyAMA / ttyCH343USB / ttyCH341USB / ttyXRUSB / ttyHS / ttyO / ttymxc / rfcomm / ttyS | `termios2` + `BOTHER` | GTK3；权限需 `dialout` |
| Windows |注册表 `HKLM\HARDWARE\DEVICEMAP\SERIALCOMM` | `DCB.BaudRate` |10/11；GUI子系统链接（无控制台） |
|其他 |返回错误「此平台暂不支持自动枚举串口」 | — | — |

端口排序：USB优先，再按 `naturalLess`（`ttyUSB2` < `ttyUSB10`，手写，无 `x/exp`依赖）。

## 12.测试

```sh
go test ./... #54 passed /3 packages，约5s
go test -race ./...
go vet ./... #另跑 go tool mygo vet（UI生命周期检查）
```

共55个 `Test`函数，其中 `TestLivePTY`默认 skip。

- `app_test.go`：行预置、入库重排、最佳跟随、端口刷新（保留 /清空 /报错）、收尾状态、closeRace、估算、
自定义速率扩展 /报错 /清空、`frameChanged`清空、`startScan`实际使用的帧格式与首个速率。
- `view_test.go`：`ui.NewTester`断言文案；点击行锁定；最小尺寸980x680；无数据 /不支持态；
帧格式与自定义速率控件存在、输入后计划更新、非法输入显示红色提示。
- `internal/baud/frame_test.go`：`Plan`顺序与去重、`CustomRates`、`ParseRates`（含中文分隔符与越界）、
 `Frame.Label`/`FrameFrom`/`Labels`往返、`Frame.Mode`。
- `internal/baud/frame_scan_test.go`：`Scan`把帧格式用于 `Open`与每次 `SetMode`，自定义速率按序扫描，零值回落8N1。
- `internal/baud/scan_test.go`：`fakePort`为每档喂不同数据（9600文本、其余噪声），验证选中9600、取消、无数据、`SetMode`失败继续。
- `internal/baud/unsupported_test.go`、`internal/ports/ports_test.go`：`EINVAL`判定与自然序排序。

环境变量钩子：

- `BAUDSCAN_SNAPSHOT` / `_DARK` / `_MIN` / `_UNSUPPORTED` →导出 PNG（README截图来源）。
- `BAUDSCAN_PTY` + `BAUDSCAN_PTY_PEER` → `TestLivePTY`走真实 `serial.Open`；需要
 `socat pty,raw,echo=0,link=/tmp/baudscan-pty-a pty,raw,echo=0,link=/tmp/baudscan-pty-b`。
 PTY原样传字节，只验驱动调用链，不验波特率判别。

## 13.构建与发布

```sh
go tool mygo dev #开发：构建开发版 App，改 .go自动重启
go tool mygo build #当前平台（macOS产出 .app + .dmg）
./build.sh # x64三平台发布，需 macOS（hdiutil）
```

`build.sh`：`mygo.json`取版本 → `go tool mygo build -platform darwin/amd64,windows/amd64,linux/amd64 -o build/release`
→复制归一化到 `dist/BaudScan-<ver>-{macos-x64.dmg,windows-x64.exe,linux-x64}` → `hdiutil verify`
→ `file`校验 PE32+ GUI / ELF64 x86-64 →写 `SHA256SUMS`。任一缺失或类型不符立即失败。
`build/`、`dist/`、`.mygo/`均在 `.gitignore`。

## 14.常见改动落点

|想改什么 |改哪里 |
| --- | --- |
|标准速率集合 /扫描顺序 | `internal/baud/scan.go`的 `Standard` / `Common`（`All`自动派生） |
|自定义速率解析 /顺序 | `internal/baud/frame.go`的 `ParseRates` / `Plan` / `CustomRates` |
|帧格式控件与标签 | `frame.go`的 `*Labels` / `FrameFrom` / `Labels()`；UI在 `view.go` `frameField` |
|采样时长选项 | `app.go` `durationOptions`（view自动读标签） |
|评分权重 /阈值 | `internal/baud/analyze.go`；同步 `analyze_test.go`与 README公式 |
|提前结束规则 | `scan.go` `withDefaults`的 `IdleAfter` / `MinSample` |
|详情面板字段 | `view.go` `detailCard` + `metric` |
|端口命名 /标签 | `internal/ports/ports_<os>.go` |
|错误文案 | `scan.go` `FriendlyError` / `Unsupported` |
|版本号 | `mygo.json`（唯一来源） |

## 15.自定义速率与帧格式（已实现）

- `baud.Frame{DataBits, Parity, StopBits}`与 `DefaultFrame`（8N1）；`Frame.Mode(rate)`产出 `serial.Mode`。
- `ScanOptions.Frame`，零值回落 `DefaultFrame`（`withDefaults`）；`Scan`的 `Open`与每次 `SetMode`都用它。
-标签与解析：`DataBitLabels`（5-8）、`ParityLabels`（无 /奇 /偶 /标记 /空格）、`StopBitLabels`（1 /1.5 /2）、
 `Frame.Label()`（8N1、8E2、7O1.5）、`FrameFrom(labels)`、`Frame.Labels()`（反向）。
-速率：`ParseRates`（分隔符含中英文逗号 /分号 /顿号 /空白，范围 `MinRate..MaxRate` =1..20000000）、
 `Plan(extra)`（Common →自定义 →其余标准升序，去重）、`CustomRates(plan)`。
- app：`plan []int`取代直接使用 `baud.All`；`customRates`文本 + `customErr`错误；
 `refreshPlan()`、`customRatesChanged()`、`frameChanged()`（改帧格式清空结果）；`defaultLabels`由 `Frame.Labels()`求得。
- `Rank(results, order)`现在接受扫描顺序表，不再隐含全局 `All`。
- `Unsupported`与 `FriendlyError`覆盖 `InvalidDataBits` / `InvalidParity` / `InvalidStopBits`。

### 15.1布局约束（踩过的坑）

`ui.Select`的触发器强制 `MinWidth = Theme.Space(35)`（默认140px），传入更小的 `Width`会被忽略，
造成列间重叠。`frameFieldWidth =140`是数据位 /校验位 /停止位三列的宽度来源，勿再调小。
五个控件同处一行：`串口设备(Grow) |数据位 |校验位 |停止位 |自定义波特率`，`Gap(10)`；
提示行与扫描状态、计划同处左侧列，省掉一整行以保证最小窗口980x680不裁切。

## 16.坑位清单

- `view.go`里的 `time`只被末尾的 `var _ = time.Second`占位使用（否则 import会报未使用）；删占位要连 import一起删。
- `ui.Select`绑定字符串值；端口下拉选项用端口名，不用 `Label`。
- `a.update(nil, fn)`在无窗口（测试）时同步执行，所以 app测试不依赖 UI线程。
- `Rank`每次 `store`都整表重排（37行，可忽略）；不要在遍历 `results`时改其顺序。
- `Result.Data`只保留前4096字节，`HexDump`再截192；不要拿它做全量分析。
- **端口互斥**：扫描中刷新 /换端口均被禁用；`refreshPorts`在端口变化时会 `resetResults`，结果永远属于当前端口。
- mygo的 `ui`是声明式重建：每帧都会重跑 `view`，别在里面做重计算或持久副作用；
需要跨帧状态就放在 `app`字段（如 `previewFor`/`previewText`）。
- **维护**：本文件会被代理自动加载；改行为时请同步更新对应小节，而不是新增平行文档。
