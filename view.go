package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/j4ckzh0u/BaudScan/internal/baud"
)

func (a *app) view(c *ui.Context) {
	ui.Column(c).Fill().Padding(22).Gap(14).Children(func() {
		a.header(c)
		a.controls(c)
		ui.Row(c).FillWidth().Grow(1).Gap(14).Children(func() {
			a.resultsCard(c)
			a.detailCard(c)
		})
		a.footer(c)
	})
}

func (a *app) header(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, "串口波特率扫描").FontSize(25).Bold()
			ui.Text(c, "只读监听设备输出，逐档测试全部波特率").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Spacer(c)
		pill(c, "只读模式 ·不发送数据", t.Success)
	})
}

func (a *app) controls(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(14).Gap(10).Background(t.Surface).Border(1, t.Border).Radius(12).Children(func() {
		ui.Column(c).FillWidth().Gap(5).Children(func() {
			fieldLabel(c, "串口设备")
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				if len(a.ports) == 0 {
					ui.Row(c).Height(30).Grow(1).Shrink(1).Padding(4, 10).Border(1, t.Border).Radius(6).
						Background(t.Background).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "未发现串口设备").FontSize(12).TextColor(t.TextMuted).SingleLine()
					})
				} else {
					options := make([]string, len(a.ports))
					for i, p := range a.ports {
						options[i] = p.Name
					}
					if ui.Select(c, &a.port, options).Label("串口设备").Disabled(a.scanning).Changed() {
						a.resetResults()
						a.status = fmt.Sprintf("已选择 %s（%s）·将测试全部 %d档", a.port, a.portLabel(a.port), len(baud.All))
						a.statusLevel = statusNormal
					}
				}
				ui.Button(c, "刷新").Width(58).Shrink(0).Disabled(a.scanning).OnClick(func() {
					a.refreshPorts(true)
				})
			})
		})

		ui.Row(c).FillWidth().AlignItems(ui.End).Gap(8).Children(func() {
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, a.coverageLine()).FontSize(11).TextColor(t.TextMuted).SingleLine()
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Box(c).Width(7).Height(7).Radius(4).Background(a.statusColor(t))
					ui.Text(c, a.status).FontSize(12).TextColor(a.statusColor(t)).SingleLine()
				})
			})

			ui.Column(c).Width(150).Shrink(0).Gap(5).Children(func() {
				fieldLabel(c, "每档采样")
				labels := make([]string, len(durationOptions))
				for i, d := range durationOptions {
					labels[i] = d.Label
				}
				ui.Select(c, &a.duration, labels).Label("每档采样时长").Disabled(a.scanning).Width(150)
			})

			ui.PrimaryButton(c, "开始扫描").Width(112).Shrink(0).Disabled(a.scanning || a.port == "").OnClick(a.startScan)
			ui.Button(c, "停止").Width(70).Shrink(0).Disabled(!a.scanning).OnClick(a.stopScan)
		})

		if a.scanning {
			ui.Progress(c, a.progress).FillWidth().Label("扫描进度")
		}
	})
}

// coverageLine states how many rates the next scan covers and how long it
// takes, so the cost of a full sweep is visible before it starts.
func (a *app) coverageLine() string {
	tested := a.doneCount()
	if tested == 0 || tested == len(a.results) {
		return fmt.Sprintf("全部 %d档 ·预计 %s", len(baud.All), shortDuration(a.estimate()))
	}
	return fmt.Sprintf("%d/%d档已测 ·全部 %d档需 %s", tested, len(baud.All), len(baud.All), shortDuration(a.estimate()))
}

func (a *app) resultsCard(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Background(t.Surface).Border(1, t.Border).Radius(12).Clip().Children(func() {
		ui.Row(c).FillWidth().Padding(14, 14, 14, 8).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "波特率测试结果").FontSize(13).Bold()
			ui.Textf(c, "· %d档 ·已测 %d", len(a.results), a.doneCount()).FontSize(11).TextColor(t.TextMuted)
			ui.Spacer(c)
			if a.scanning {
				ui.Textf(c, "%.0f%%", a.progress*100).FontSize(11).TextColor(t.Accent)
			}
		})

		if len(a.results) == 0 {
			a.emptyResults(c)
			return
		}
		a.table.Selected = &a.selected
		a.table.Key = func(row int) any { return a.results[row].Baud }
		a.table.Label = func(row int) string {
			r := a.results[row]
			if r.Pending {
				return fmt.Sprintf("%d baud，待测", r.Baud)
			}
			return fmt.Sprintf("%d baud，%s", r.Baud, baud.KindLabel(r.Analysis.Kind))
		}
		columns := []ui.TableColumn{
			{Title: "波特率", Width: 92, Align: ui.End},
			{Title: "评分", Width: 58, Align: ui.End},
			{Title: "数据", Width: 74, Align: ui.End},
			{Title: "类型", Width: 118},
			{Title: "样本", MinWidth: 150},
		}
		if ui.Table(c, &a.table, columns, len(a.results), func(row, col int) {
			a.tableCell(c, row, col)
		}).Grow(1).Changed() {
			// A click or a key chooses a row: the detail panel follows the
			// user, not the best score, until the next scan.
			a.manualPick = true
		}

		ui.Row(c).FillWidth().Padding(9, 14).Children(func() {
			ui.Text(c, "按评分排序 ·待测项排在最后 ·点击一行查看原始字节").
				FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func (a *app) tableCell(c *ui.Context, row, col int) {
	t := c.Theme()
	r := a.results[row]
	if col == 0 {
		ui.Text(c, baud.BaudLabel(r.Baud)).Font("monospace").FontSize(12).Bold()
		return
	}
	if col == 1 {
		if r.Pending || r.Err != "" {
			ui.Text(c, "—").TextColor(t.TextMuted)
			return
		}
		ui.Textf(c, "%d", r.Analysis.Score).FontSize(12)
		return
	}
	if col == 2 {
		if r.Pending {
			ui.Text(c, "—").TextColor(t.TextMuted)
			return
		}
		ui.Text(c, baud.FormatBytes(r.Bytes)).FontSize(12)
		return
	}
	if col == 3 {
		if r.Pending {
			ui.Text(c, "待测").FontSize(12).TextColor(t.TextMuted)
			return
		}
		label := baud.KindLabel(r.Analysis.Kind)
		if r.Unsupported {
			label = "不支持"
		} else if r.Err != "" {
			label = "错误"
		} else if row == 0 && r.Analysis.Score >= 50 {
			label += " ·最佳"
		}
		ui.Text(c, label).FontSize(12).SingleLine()
		return
	}
	sample := ""
	if r.Unsupported {
		sample = "驱动不支持该速率"
	} else if r.Err != "" {
		sample = r.Err
	} else if !r.Pending && len(r.Data) > 0 {
		sample = baud.Preview(r.Data, 100)
	}
	ui.Text(c, sample).Font("monospace").FontSize(11).SingleLine()
}

func (a *app) emptyResults(c *ui.Context) {
	t := c.Theme()
	title, subtitle := "准备扫描", "选择串口设备，然后点击开始扫描"
	switch {
	case a.scanning:
		title = "正在监听串口"
		subtitle = "正在逐个波特率采样，请稍候"
	case len(a.ports) == 0:
		title = "未发现串口设备"
		subtitle = "接入设备后点击刷新"
	}
	ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
		ui.Text(c, "⌁").FontSize(42).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(14).FontWeight(600)
		ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) detailCard(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(404).FillHeight().Background(t.Surface).Border(1, t.Border).Radius(12).
		Padding(16).Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "检测详情").FontSize(13).Bold()
			ui.Spacer(c)
			if a.scanning {
				ui.Text(c, "采样中").FontSize(11).TextColor(t.Accent)
			}
		})

		if a.selected < 0 || a.selected >= len(a.results) {
			a.emptyDetail(c)
			return
		}
		r := a.results[a.selected]
		if r.Pending {
			a.pendingDetail(c, r)
			return
		}
		if r.Unsupported {
			ui.Text(c, "设备不支持").FontSize(16).Bold().TextColor(t.Warning)
			ui.Text(c, "串口驱动拒绝了这个波特率，扫描会自动跳过。").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, "较低或常见的速率仍可正常测试，其它档位不受影响。").FontSize(11).TextColor(t.TextMuted)
			return
		}
		if r.Err != "" {
			ui.Text(c, "无法读取").FontSize(16).Bold().TextColor(t.Danger)
			ui.Text(c, r.Err).FontSize(12).TextColor(t.Danger)
			ui.Spacer(c)
			ui.Text(c, "换一个波特率，或检查设备连接后重新扫描。").FontSize(11).TextColor(t.TextMuted)
			return
		}

		ui.Row(c).FillWidth().AlignItems(ui.Baseline).Gap(7).Children(func() {
			ui.Text(c, baud.BaudLabel(r.Baud)).Font("monospace").FontSize(32).Bold()
			ui.Text(c, "baud").FontSize(11).TextColor(t.TextMuted)
			ui.Spacer(c)
			pill(c, confidenceLabel(a, a.selected), confidenceColor(a, a.selected, t))
		})
		ui.Text(c, a.scoreLine()).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, fmt.Sprintf("%s · %s · %d行", baud.KindLabel(r.Analysis.Kind), baud.FormatBytes(r.Bytes), r.Analysis.Lines)).
			FontSize(11).TextColor(t.TextMuted)

		ui.Row(c).FillWidth().Gap(8).Children(func() {
			metric(c, "可读字符", fmt.Sprintf("%.0f%%", r.Analysis.TextRatio*100), "")
			metric(c, "文本行", fmt.Sprintf("%d", r.Analysis.Lines), fmt.Sprintf("可信 %d", r.Analysis.GoodLines))
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			sub := "无周期"
			if r.Analysis.RepeatLag > 0 {
				sub = fmt.Sprintf("周期 %d B", r.Analysis.RepeatLag)
			}
			metric(c, "重复结构", fmt.Sprintf("%.2f", r.Analysis.Repeat), sub)
			metric(c, "字节熵", fmt.Sprintf("%.1f", r.Analysis.Entropy), "bit /字节")
		})

		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "原始字节").FontSize(12).FontWeight(600)
			ui.Spacer(c)
			ui.Textf(c, "前 %d字节", minInt(len(r.Data), 192)).FontSize(11).TextColor(t.TextMuted)
		})

		key := fmt.Sprintf("%s/%d/%d", a.port, r.Baud, r.Bytes)
		resetPreview := a.previewFor != key
		if resetPreview {
			a.previewText = baud.HexDump(r.Data, 192)
			a.previewFor = key
		}
		preview := ui.TextArea(c, &a.previewText).ReadOnly(true).Font("monospace").FontSize(11).
			Grow(1).FillHeight().Label("原始字节")
		if resetPreview {
			preview.SetTextSelection(0, 0)
		}

		ui.Text(c, a.verdict(r)).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) pendingDetail(c *ui.Context, r baud.Result) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Baseline).Gap(7).Children(func() {
		ui.Text(c, baud.BaudLabel(r.Baud)).Font("monospace").FontSize(32).Bold().TextColor(t.TextMuted)
		ui.Text(c, "baud").FontSize(11).TextColor(t.TextMuted)
		ui.Spacer(c)
		pill(c, "待测", t.TextMuted)
	})
	ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
		ui.Text(c, "⌁").FontSize(38).TextColor(t.TextMuted)
		ui.Text(c, "这一档还没测试").FontSize(13).FontWeight(600)
		ui.Text(c, "扫描会按顺序走完全部档位").FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) emptyDetail(c *ui.Context) {
	t := c.Theme()
	title, subtitle := "尚未选择候选", "扫描开始后，点击一行查看原始字节"
	switch {
	case a.scanning:
		title = "正在监听串口"
		subtitle = "结果会随扫描进度出现"
	case len(a.results) > 0 && !a.hasData():
		title = "还没有可用样本"
		subtitle = "开始扫描后将测试全部档位"
	}
	ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
		ui.Text(c, "⌁").FontSize(38).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(13).FontWeight(600)
		ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) footer(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, fmt.Sprintf("只读扫描，不会向设备发送任何字节 ·逐档测试 %d个标准波特率（8N1）·单次全扫约 %s",
		len(baud.All), shortDuration(a.estimate()))).
		FontSize(11).TextColor(t.TextMuted)
}

func (a *app) scoreLine() string {
	if a.selected < 0 || a.selected >= len(a.results) {
		return ""
	}
	r := a.results[a.selected]
	line := fmt.Sprintf("评分 %d /100", r.Analysis.Score)
	switch {
	case a.selected == 0 && len(a.results) > 1 && a.margin() > 0:
		line += fmt.Sprintf(" ·领先第二名 %d分", a.margin())
	case a.selected > 0:
		line += fmt.Sprintf(" ·比最佳 %d baud低 %d分", a.results[0].Baud, a.results[0].Analysis.Score-r.Analysis.Score)
	}
	return line
}

func (a *app) verdict(r baud.Result) string {
	switch {
	case a.selected > 0 && a.results[0].Analysis.Score > r.Analysis.Score:
		return fmt.Sprintf("不是最佳候选；最佳为 %d baud（%d分）。", a.results[0].Baud, a.results[0].Analysis.Score)
	case a.selected == 0 && a.closeRace():
		return "前两名评分接近，建议对比两组原始字节后确认。"
	}
	switch r.Analysis.Kind {
	case baud.KindText:
		return "样本是可读文本，这个波特率很可能正确。"
	case baud.KindBinary:
		return "样本有重复的帧结构，但内容不可读；请结合设备协议确认。"
	case baud.KindConstant:
		return "数据恒定，不包含可判断波特率的信息。"
	case baud.KindNoise:
		return "样本缺乏可读文本和重复结构，可能不是正确的波特率。"
	case baud.KindNoData:
		return "采样期间没有收到数据；设备可能需要主动上报，或采样时间太短。"
	default:
		return "样本不足，无法判断。"
	}
}

func metric(c *ui.Context, label, value, sub string) {
	t := c.Theme()
	ui.Column(c).Grow(1).Padding(9, 10).Background(t.Background).Radius(8).Gap(1).Children(func() {
		ui.Text(c, label).FontSize(10).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(16).FontWeight(600)
		if sub != "" {
			ui.Text(c, sub).FontSize(10).TextColor(t.TextMuted).SingleLine()
		}
	})
}

func fieldLabel(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(11).FontWeight(600).TextColor(c.Theme().TextMuted)
}

func pill(c *ui.Context, text string, color ui.Color) {
	t := c.Theme()
	alpha := float32(0.12)
	if t.Dark {
		alpha = 0.18
	}
	ui.Text(c, text).FontSize(10).FontWeight(600).TextColor(color).
		Padding(3, 9).Radius(20).Background(ui.RGBA(color.R, color.G, color.B, alpha))
}

func (a *app) statusColor(t *ui.Theme) ui.Color {
	switch a.statusLevel {
	case statusGood:
		return t.Success
	case statusWarn:
		return t.Warning
	case statusBad:
		return t.Danger
	default:
		return t.TextMuted
	}
}

func confidenceLabel(a *app, row int) string {
	r := a.results[row]
	if row == 0 && a.closeRace() {
		return "存疑"
	}
	switch {
	case r.Analysis.Score >= 75:
		return "高置信度"
	case r.Analysis.Score >= 50:
		return "中等置信度"
	default:
		return "低置信度"
	}
}

func confidenceColor(a *app, row int, t *ui.Theme) ui.Color {
	if row == 0 && a.closeRace() {
		return t.Warning
	}
	switch {
	case a.results[row].Analysis.Score >= 75:
		return t.Success
	case a.results[row].Analysis.Score >= 50:
		return t.Warning
	default:
		return t.TextMuted
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = time.Second
