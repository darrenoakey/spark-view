// Package ui implements the Gio-based Spark View dashboard.
package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sync"
	"time"

	"sparkview/pkg/arbiter"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Design tokens — dark theme with vibrant accents.
var (
	bgColor      = color.NRGBA{R: 0x0f, G: 0x0f, B: 0x0f, A: 0xff}
	headerBGClr  = color.NRGBA{R: 0x18, G: 0x18, B: 0x18, A: 0xff}
	rowAltColor  = color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff}
	separatorClr = color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff}
	statusBarBG  = color.NRGBA{R: 0x12, G: 0x12, B: 0x18, A: 0xff}

	textPrimary   = color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}
	textSecondary = color.NRGBA{R: 0xa8, G: 0xa8, B: 0xa8, A: 0xff}
	textMuted     = color.NRGBA{R: 0x60, G: 0x60, B: 0x70, A: 0xff}

	// Row highlight colors (subtle background tints)
	rowLoadedBG = color.NRGBA{R: 0x00, G: 0x2a, B: 0x1a, A: 0xff} // dark green tint
	rowActiveBG = color.NRGBA{R: 0x2a, G: 0x1a, B: 0x00, A: 0xff} // dark amber tint

	// Accent palette
	accentCyan   = color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0xff}
	accentGreen  = color.NRGBA{R: 0x5c, G: 0xb8, B: 0x5c, A: 0xff}
	accentOrange = color.NRGBA{R: 0xff, G: 0xb8, B: 0x4d, A: 0xff}
	accentRed    = color.NRGBA{R: 0xff, G: 0x5c, B: 0x5c, A: 0xff}
	accentPurple = color.NRGBA{R: 0xb4, G: 0x7a, B: 0xff, A: 0xff}

	// Gauge colors
	gaugeTrack = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x28, A: 0xff}
	gaugeLow   = color.NRGBA{R: 0x00, G: 0xe6, B: 0x96, A: 0xff}
	gaugeMid   = color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0xff}
	gaugeHigh  = color.NRGBA{R: 0xff, G: 0x9f, B: 0x43, A: 0xff}
	gaugeFull  = color.NRGBA{R: 0xff, G: 0x4d, B: 0x6a, A: 0xff}

	dotLoaded   = color.NRGBA{R: 0x00, G: 0xe6, B: 0x96, A: 0xff}
	dotUnloaded = color.NRGBA{R: 0x44, G: 0x44, B: 0x55, A: 0xff}
)

// Column widths: Model (flexed), State, VRAM, Active, Queued, Idle
var colWidths = [5]unit.Dp{0, 90, 80, 80, 80} // model name is flexed

// App holds the Spark View application state.
type App struct {
	theme  *material.Theme
	win    *app.Window
	client *arbiter.Client

	mu          sync.Mutex
	status      arbiter.Status
	lastRefresh time.Time
	lastErr     error
	connected   bool

	list widget.List
}

// NewApp creates a new Spark View application.
func NewApp(win *app.Window, client *arbiter.Client) *App {
	th := material.NewTheme()
	a := &App{
		theme:  th,
		win:    win,
		client: client,
	}
	a.list.Axis = layout.Vertical
	return a
}

// Refresh fetches fresh data from the Arbiter server. Call from a goroutine.
func (a *App) Refresh() {
	status, err := a.client.PS()

	a.mu.Lock()
	if err != nil {
		a.lastErr = err
		a.connected = false
	} else {
		a.status = status
		a.lastErr = nil
		a.connected = true
	}
	a.lastRefresh = time.Now()
	a.mu.Unlock()

	a.win.Invalidate()
}

// Layout renders the full UI frame.
func (a *App) Layout(gtx layout.Context) layout.Dimensions {
	paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())

	a.mu.Lock()
	status := a.status
	connected := a.connected
	lastErr := a.lastErr
	lastRefresh := a.lastRefresh
	a.mu.Unlock()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Table header
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutHeader(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutSeparator(gtx, separatorClr)
		}),
		// Table body
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !connected && lastRefresh.IsZero() {
				return a.layoutConnecting(gtx)
			}
			return a.layoutTable(gtx, status)
		}),
		// Bottom separator
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutSeparator(gtx, separatorClr)
		}),
		// Status bar
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutStatusBar(gtx, status, connected, lastErr, lastRefresh)
		}),
	)
}

func (a *App) layoutHeader(gtx layout.Context) layout.Dimensions {
	headerH := gtx.Dp(unit.Dp(32))
	totalW := gtx.Constraints.Max.X

	paint.FillShape(gtx.Ops, headerBGClr, clip.Rect{Max: image.Pt(totalW, headerH)}.Op())

	type headerCol struct {
		label string
		align text.Alignment
	}
	cols := []headerCol{
		{"Model", text.Start},
		{"State", text.Start},
		{"VRAM", text.End},
		{"Active", text.End},
		{"Queued", text.End},
	}

	nameW := totalW
	for i := 1; i < len(colWidths); i++ {
		nameW -= gtx.Dp(colWidths[i])
	}

	x := 0
	for i, col := range cols {
		var colW int
		if i == 0 {
			colW = nameW
		} else {
			colW = gtx.Dp(colWidths[i])
		}

		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		gtxCol := gtx
		gtxCol.Constraints = layout.Exact(image.Pt(colW, headerH))

		layout.Inset{
			Left: unit.Dp(16), Right: unit.Dp(16),
			Top: unit.Dp(8),
		}.Layout(gtxCol, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, col.label)
			l.Font.Weight = font.Medium
			l.TextSize = unit.Sp(11)
			l.Color = textMuted
			l.Alignment = col.align
			return l.Layout(gtx)
		})
		offset.Pop()
		x += colW
	}

	return layout.Dimensions{Size: image.Pt(totalW, headerH)}
}

func (a *App) layoutConnecting(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Body1(a.theme, "Connecting to Arbiter...")
		l.Color = textMuted
		l.TextSize = unit.Sp(14)
		return l.Layout(gtx)
	})
}

func (a *App) layoutTable(gtx layout.Context, status arbiter.Status) layout.Dimensions {
	return material.List(a.theme, &a.list).Layout(gtx, len(status.Models), func(gtx layout.Context, index int) layout.Dimensions {
		return a.layoutRow(gtx, status.Models[index], index)
	})
}

func (a *App) layoutRow(gtx layout.Context, m arbiter.Model, index int) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(36))
	totalW := gtx.Constraints.Max.X

	// Row background: active jobs > loaded > alternating
	switch {
	case m.ActiveJobs > 0:
		paint.FillShape(gtx.Ops, rowActiveBG, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
	case m.State == "loaded":
		paint.FillShape(gtx.Ops, rowLoadedBG, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
	default:
		if index%2 == 0 {
			paint.FillShape(gtx.Ops, rowAltColor, clip.Rect{Max: image.Pt(totalW, rowH)}.Op())
		}
	}

	nameW := totalW
	for i := 1; i < len(colWidths); i++ {
		nameW -= gtx.Dp(colWidths[i])
	}

	// Determine row colors based on state
	nameColor := textSecondary
	if m.State == "loaded" {
		nameColor = textPrimary
	}

	stateColor := textMuted
	stateText := m.State
	if m.State == "loaded" {
		stateColor = accentGreen
	}

	activeColor := textMuted
	activeText := fmt.Sprintf("%d", m.ActiveJobs)
	if m.ActiveJobs > 0 {
		activeColor = accentOrange
	}

	queuedColor := textMuted
	queuedText := fmt.Sprintf("%d", m.QueuedJobs)
	if m.QueuedJobs > 0 {
		queuedColor = accentPurple
	}

	vramText := fmt.Sprintf("%.0f GB", m.MemoryGB)
	vramColor := textSecondary
	if m.State != "loaded" {
		vramColor = textMuted
	}

	type colData struct {
		val   string
		width int
		align text.Alignment
		color color.NRGBA
		bold  bool
	}

	columns := []colData{
		{m.ID, nameW, text.Start, nameColor, true},
		{stateText, gtx.Dp(colWidths[1]), text.Start, stateColor, false},
		{vramText, gtx.Dp(colWidths[2]), text.End, vramColor, false},
		{activeText, gtx.Dp(colWidths[3]), text.End, activeColor, false},
		{queuedText, gtx.Dp(colWidths[4]), text.End, queuedColor, false},
	}

	x := 0
	for _, col := range columns {
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		gtxCol := gtx
		gtxCol.Constraints = layout.Exact(image.Pt(col.width, rowH))

		layout.Inset{
			Left: unit.Dp(16), Right: unit.Dp(16),
			Top: unit.Dp(10),
		}.Layout(gtxCol, func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, col.val)
			l.Color = col.color
			l.TextSize = unit.Sp(12)
			l.Alignment = col.align
			if col.bold {
				l.Font.Weight = font.Medium
			}
			l.MaxLines = 1
			return l.Layout(gtx)
		})

		offset.Pop()
		x += col.width
	}

	// Row separator
	sepOff := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), rowH-1)).Push(gtx.Ops)
	sepW := totalW - gtx.Dp(unit.Dp(32))
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff},
		clip.Rect{Max: image.Pt(sepW, 1)}.Op())
	sepOff.Pop()

	return layout.Dimensions{Size: image.Pt(totalW, rowH)}
}

func (a *App) layoutStatusBar(gtx layout.Context, status arbiter.Status, connected bool, lastErr error, lastRefresh time.Time) layout.Dimensions {
	barH := gtx.Dp(unit.Dp(36))
	totalW := gtx.Constraints.Max.X

	paint.FillShape(gtx.Ops, statusBarBG, clip.Rect{Max: image.Pt(totalW, barH)}.Op())

	return layout.Inset{
		Left: unit.Dp(16), Right: unit.Dp(16),
		Top: unit.Dp(8), Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			// Left side: VRAM gauge
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// Connection dot
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dotSize := gtx.Dp(unit.Dp(6))
						dotColor := dotUnloaded
						if connected {
							dotColor = dotLoaded
						}
						r := clip.Ellipse{Max: image.Pt(dotSize, dotSize)}.Op(gtx.Ops)
						paint.FillShape(gtx.Ops, dotColor, r)
						return layout.Dimensions{Size: image.Pt(dotSize, dotSize)}
					}),
					// VRAM label
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Body2(a.theme, "VRAM")
							l.Color = textMuted
							l.TextSize = unit.Sp(10)
							return l.Layout(gtx)
						})
					}),
					// VRAM gauge
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Dp(unit.Dp(100))
							gtx.Constraints.Max.X = gtx.Dp(unit.Dp(100))
							return a.layoutGaugeBar(gtx, status.VRAMUsedGB, status.VRAMBudgetGB)
						})
					}),
					// VRAM numbers
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Body2(a.theme, fmt.Sprintf("%.0f / %.0f GB", status.VRAMUsedGB, status.VRAMBudgetGB))
							l.Color = textSecondary
							l.TextSize = unit.Sp(10)
							return l.Layout(gtx)
						})
					}),
				)
			}),
			// Right side: queue stats
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.layoutStatusStat(gtx, "run", status.Queue.Running, accentGreen)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutStatusStat(gtx, "queue", status.Queue.Queued, accentPurple)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutStatusStat(gtx, "done", status.Queue.Completed, accentCyan)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutStatusStat(gtx, "fail", status.Queue.Failed, accentRed)
						})
					}),
					// Update time
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if lastRefresh.IsZero() {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							var statusText string
							if connected {
								ago := time.Since(lastRefresh).Truncate(time.Second)
								statusText = fmt.Sprintf("%s ago", ago)
							} else if lastErr != nil {
								statusText = "offline"
							} else {
								statusText = "..."
							}
							l := material.Body2(a.theme, statusText)
							l.Color = textMuted
							l.TextSize = unit.Sp(10)
							return l.Layout(gtx)
						})
					}),
				)
			}),
		)
	})
}

func (a *App) layoutStatusStat(gtx layout.Context, label string, count int, clr color.NRGBA) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			numColor := textMuted
			if count > 0 {
				numColor = clr
			}
			l := material.Body2(a.theme, fmt.Sprintf("%d", count))
			l.Color = numColor
			l.Font.Weight = font.Bold
			l.TextSize = unit.Sp(11)
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(a.theme, label)
				l.Color = textMuted
				l.TextSize = unit.Sp(10)
				return l.Layout(gtx)
			})
		}),
	)
}

func (a *App) layoutGaugeBar(gtx layout.Context, used, budget float64) layout.Dimensions {
	barH := gtx.Dp(unit.Dp(6))
	barW := gtx.Constraints.Max.X
	radius := barH / 2

	trackRect := clip.RRect{
		Rect: image.Rect(0, 0, barW, barH),
		NE:   radius, NW: radius, SE: radius, SW: radius,
	}
	paint.FillShape(gtx.Ops, gaugeTrack, trackRect.Op(gtx.Ops))

	pct := 0.0
	if budget > 0 {
		pct = used / budget
	}
	if pct > 1 {
		pct = 1
	}
	fillW := int(math.Round(pct * float64(barW)))
	if fillW > 0 {
		fillColor := gaugeColor(pct)
		fillRect := clip.RRect{
			Rect: image.Rect(0, 0, fillW, barH),
			NE:   radius, NW: radius, SE: radius, SW: radius,
		}
		paint.FillShape(gtx.Ops, fillColor, fillRect.Op(gtx.Ops))
	}

	return layout.Dimensions{Size: image.Pt(barW, barH)}
}

// layoutSeparator draws a horizontal line.
func layoutSeparator(gtx layout.Context, clr color.NRGBA) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	w := gtx.Constraints.Max.X
	paint.FillShape(gtx.Ops, clr, clip.Rect{Max: image.Pt(w, h)}.Op())
	return layout.Dimensions{Size: image.Pt(w, h)}
}

// gaugeColor returns a color based on VRAM usage percentage.
func gaugeColor(pct float64) color.NRGBA {
	switch {
	case pct < 0.5:
		return gaugeLow
	case pct < 0.7:
		return gaugeMid
	case pct < 0.9:
		return gaugeHigh
	default:
		return gaugeFull
	}
}

// formatDuration formats seconds into a human-readable duration.
func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
