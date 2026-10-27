package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"baudscan/internal/baud"
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
			ui.Text(c, "只读监听设备输出，自动找出正确的波特率").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Spacer(c)
		pill(c, "只读模式 · 不发送数据", t.Success)
	})
}

func (a *app) controls(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(14).Gap(11).Background(t.Surface).Border(1, t.Border).Radius(12).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.End).Gap(10).Children(func() {
			ui.Column(c).Grow(1).MinWidth(210).Gap(5).Children(func() {
				fieldLabel(c, "串口设备")
				ui.Row(c).FillWidth().Gap(6).Children(func() {
					if len(a.ports) == 0 {
						ui.Row(c).Height(30).Grow(1).Padding(4, 10).Border(1, t.Border).Radius(6).
							Background(t.Background).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "未发现串口设备").FontSize(12).TextColor(t.TextMuted).SingleLine()
						})
					} else {
						options := make([]string, len(a.ports))
						for i, p := range a.ports {
							options[i] = p.Name
						}
						if ui.Select(c, &a.port, options).Label("串口设备").Disabled(a.scanning).Changed() {
							a.results = nil
							a.selected = -1
							a.previewFor = ""
							a.status = fmt.Sprintf("已选择 %s（%s）", a.port, a.portLabel(a.port))
							a.statusLevel = statusNormal
						}
					}
					ui.Button(c, "刷新").Width(58).Disabled(a.scanning).OnClick(func() {
						a.refreshPorts(true)
					})
				})
			})

			ui.Column(c).Width(132).Gap(5).Children(func() {
				fieldLabel(c, "扫描范围")
				ui.Select(c, &a.preset, presetNames()).Label("扫描范围").Disabled(a.scanning).Width(132)
			})

			ui.Column(c).Width(126).Gap(5).Children(func() {
				fieldLabel(c, "每档采样")
				labels := make([]string, len(durationOptions))
				for i, d := range durationOptions {
					labels[i] = d.Label
				}
				ui.Select(c, &a.duration, labels).Label("每档采样时长").Disabled(a.scanning).Width(126)
			})

			ui.PrimaryButton(c, "开始扫描").Width(108).Disabled(a.scanning || a.port == "").OnClick(a.startScan)
			ui.Button(c, "停止").Width(66).Disabled(!a.scanning).OnClick(a.stopScan)
		})

		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Box(c).Width(7).Height(7).Radius(4).Background(a.statusColor(t))
			ui.Text(c, a.status).FontSize(12).TextColor(a.statusColor(t)).Grow(1)
		})

		if a.scanning {
			ui.Progress(c, a.progress).FillWidth().Label("扫描进度")
		}
	})
}

func (a *app) resultsCard(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Background(t.Surface).Border(1, t.Border).Radius(12).Clip().Children(func() {
		ui.Row(c).FillWidth().Padding(14, 14, 14, 8).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "候选波特率").FontSize(13).Bold()
			if len(a.results) > 0 {
				ui.Textf(c, "· %d 个", len(a.results)).FontSize(11).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			if a.scanning {
				ui.Text(c, "扫描中").FontSize(11).TextColor(t.Accent)
			}
		})

		if len(a.results) == 0 {
			a.emptyResults(c)
		} else {
			ui.Column(c).Grow(1).Padding(0, 6).Children(func() {
				a.table.Selected = &a.selected
				a.table.Key = func(row int) any { return a.results[row].Baud }
				a.table.Label = func(row int) string {
					r := a.results[row]
					return fmt.Sprintf("%d baud，%s", r.Baud, baud.KindLabel(r.Analysis.Kind))
				}
				columns := []ui.TableColumn{
					{Title: "波特率", Width: 92, Align: ui.End},
					{Title: "评分", Width: 58, Align: ui.End},
					{Title: "数据", Width: 80, Align: ui.End},
					{Title: "类型", Width: 118},
					{Title: "样本", MinWidth: 150},
				}
				ui.Table(c, &a.table, columns, len(a.results), func(row, col int) {
					a.tableCell(c, row, col)
				}).Grow(1)
			})
		}

		if len(a.results) > 0 {
			ui.Row(c).FillWidth().Padding(9, 14).Children(func() {
				ui.Text(c, "按评分排序 · 点击一行查看原始字节").FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
}

func (a *app) tableCell(c *ui.Context, row, col int) {
	r := a.results[row]
	switch col {
	case 0:
		ui.Text(c, baud.BaudLabel(r.Baud)).Font("monospace").FontSize(12).Bold()
	case 1:
		if r.Err != "" {
			ui.Text(c, "—")
		} else {
			ui.Textf(c, "%d", r.Analysis.Score).FontSize(12)
		}
	case 2:
		ui.Text(c, baud.FormatBytes(r.Bytes)).FontSize(12)
	case 3:
		label := baud.KindLabel(r.Analysis.Kind)
		if r.Err != "" {
			label = "错误"
		} else if row == 0 && r.Analysis.Score >= 50 {
			label += " · 最佳"
		}
		ui.Text(c, label).FontSize(12).SingleLine()
	case 4:
		sample := ""
		switch {
		case r.Err != "":
			sample = r.Err
		case len(r.Data) > 0:
			sample = baud.Preview(r.Data, 100)
		}
		ui.Text(c, sample).Font("monospace").FontSize(11).SingleLine()
	}
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
		ui.Text(c, fmt.Sprintf("%s · %s · %d 行", baud.KindLabel(r.Analysis.Kind), baud.FormatBytes(r.Bytes), r.Analysis.Lines)).
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
			metric(c, "字节熵", fmt.Sprintf("%.1f", r.Analysis.Entropy), "bit / 字节")
		})

		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "原始字节").FontSize(12).FontWeight(600)
			ui.Spacer(c)
			ui.Textf(c, "前 %d 字节", minInt(len(r.Data), 192)).FontSize(11).TextColor(t.TextMuted)
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

func (a *app) emptyDetail(c *ui.Context) {
	t := c.Theme()
	title, subtitle := "尚未选择候选", "扫描完成后，点击一行查看原始字节"
	switch {
	case a.scanning:
		title = "正在监听串口"
		subtitle = "结果会随扫描进度出现"
	case len(a.results) > 0 && !a.hasData():
		title = "没有收到数据"
		subtitle = "试试更长的每档采样，或确认设备会主动输出"
	}
	ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
		ui.Text(c, "⌁").FontSize(38).TextColor(t.TextMuted)
		ui.Text(c, title).FontSize(13).FontWeight(600)
		ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *app) footer(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "只读扫描，不会向设备发送任何字节 · 自动识别对加密或随机二进制流不可靠").
		FontSize(11).TextColor(t.TextMuted)
}

func (a *app) scoreLine() string {
	if a.selected < 0 || a.selected >= len(a.results) {
		return ""
	}
	r := a.results[a.selected]
	line := fmt.Sprintf("评分 %d / 100", r.Analysis.Score)
	switch {
	case a.selected == 0 && len(a.results) > 1 && a.margin() > 0:
		line += fmt.Sprintf(" · 领先第二名 %d 分", a.margin())
	case a.selected == 0 && len(a.results) == 1:
		line += " · 只测试了一档"
	case a.selected > 0:
		line += fmt.Sprintf(" · 比最佳 %d baud 低 %d 分", a.results[0].Baud, a.results[0].Analysis.Score-r.Analysis.Score)
	}
	return line
}

func (a *app) verdict(r baud.Result) string {
	switch {
	case a.selected > 0:
		return fmt.Sprintf("不是最佳候选；最佳为 %d baud（%d 分）。", a.results[0].Baud, a.results[0].Analysis.Score)
	case a.closeRace():
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

func presetNames() []string {
	names := make([]string, len(baud.Presets))
	for i, p := range baud.Presets {
		names[i] = p.Name
	}
	return names
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
