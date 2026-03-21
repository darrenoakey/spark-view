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
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Design tokens — dark theme with vibrant accents.
var (
	bgColor      = color.NRGBA{R: 0x0a, G: 0x0a, B: 0x0e, A: 0xff}
	surfaceColor = color.NRGBA{R: 0x14, G: 0x14, B: 0x1c, A: 0xff}
	cardColor    = color.NRGBA{R: 0x18, G: 0x18, B: 0x24, A: 0xff}
	borderColor  = color.NRGBA{R: 0x2a, G: 0x2a, B: 0x3a, A: 0xff}
	separatorClr = color.NRGBA{R: 0x22, G: 0x22, B: 0x30, A: 0xff}

	textPrimary   = color.NRGBA{R: 0xe8, G: 0xe8, B: 0xf0, A: 0xff}
	textSecondary = color.NRGBA{R: 0x9a, G: 0x9a, B: 0xb0, A: 0xff}
	textMuted     = color.NRGBA{R: 0x60, G: 0x60, B: 0x78, A: 0xff}
	textBright    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	// Accent palette
	accentCyan    = color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0xff}
	accentGreen   = color.NRGBA{R: 0x00, G: 0xe6, B: 0x96, A: 0xff}
	accentOrange  = color.NRGBA{R: 0xff, G: 0x9f, B: 0x43, A: 0xff}
	accentRed     = color.NRGBA{R: 0xff, G: 0x4d, B: 0x6a, A: 0xff}
	accentPurple  = color.NRGBA{R: 0xb4, G: 0x7a, B: 0xff, A: 0xff}
	accentDimCyan = color.NRGBA{R: 0x00, G: 0x6a, B: 0x80, A: 0xff}

	// Gauge colors
	gaugeTrack = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x28, A: 0xff}
	gaugeLow   = color.NRGBA{R: 0x00, G: 0xe6, B: 0x96, A: 0xff} // green
	gaugeMid   = color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0xff} // cyan
	gaugeHigh  = color.NRGBA{R: 0xff, G: 0x9f, B: 0x43, A: 0xff} // orange
	gaugeFull  = color.NRGBA{R: 0xff, G: 0x4d, B: 0x6a, A: 0xff} // red

	// Status dots
	dotLoaded   = color.NRGBA{R: 0x00, G: 0xe6, B: 0x96, A: 0xff}
	dotUnloaded = color.NRGBA{R: 0x44, G: 0x44, B: 0x55, A: 0xff}
)

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
	// Fill background
	paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())

	a.mu.Lock()
	status := a.status
	connected := a.connected
	lastErr := a.lastErr
	lastRefresh := a.lastRefresh
	a.mu.Unlock()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Title bar
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutTitleBar(gtx, connected, lastErr, lastRefresh)
		}),
		// Separator
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutSeparator(gtx, separatorClr)
		}),
		// Scrollable content
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !connected && lastRefresh.IsZero() {
				return a.layoutConnecting(gtx)
			}
			return a.layoutContent(gtx, status)
		}),
	)
}

func (a *App) layoutTitleBar(gtx layout.Context, connected bool, lastErr error, lastRefresh time.Time) layout.Dimensions {
	return layout.Inset{
		Top: unit.Dp(16), Bottom: unit.Dp(12),
		Left: unit.Dp(24), Right: unit.Dp(24),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Lightning bolt prefix
						l := material.Body1(a.theme, "SPARK")
						l.Color = accentCyan
						l.Font.Weight = font.Bold
						l.TextSize = unit.Sp(18)
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Body1(a.theme, "VIEW")
							l.Color = textMuted
							l.Font.Weight = font.Light
							l.TextSize = unit.Sp(18)
							return l.Layout(gtx)
						})
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var statusText string
				var statusColor color.NRGBA
				if connected {
					if lastRefresh.IsZero() {
						statusText = "connecting..."
						statusColor = textMuted
					} else {
						ago := time.Since(lastRefresh).Truncate(time.Second)
						statusText = fmt.Sprintf("updated %s ago", ago)
						statusColor = textMuted
					}
				} else if lastErr != nil {
					statusText = "offline"
					statusColor = accentRed
				} else {
					statusText = "connecting..."
					statusColor = textMuted
				}

				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Status dot
						dotSize := gtx.Dp(unit.Dp(6))
						dotColor := dotUnloaded
						if connected {
							dotColor = dotLoaded
						}
						r := clip.Ellipse{Max: image.Pt(dotSize, dotSize)}.Op(gtx.Ops)
						paint.FillShape(gtx.Ops, dotColor, r)
						return layout.Dimensions{Size: image.Pt(dotSize, dotSize)}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							l := material.Body2(a.theme, statusText)
							l.Color = statusColor
							l.TextSize = unit.Sp(11)
							return l.Layout(gtx)
						})
					}),
				)
			}),
		)
	})
}

func (a *App) layoutConnecting(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Body1(a.theme, "Connecting to Arbiter...")
		l.Color = textMuted
		l.TextSize = unit.Sp(14)
		return l.Layout(gtx)
	})
}

func (a *App) layoutContent(gtx layout.Context, status arbiter.Status) layout.Dimensions {
	return layout.Inset{
		Left: unit.Dp(24), Right: unit.Dp(24),
		Top: unit.Dp(16), Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// VRAM gauge section
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutVRAMSection(gtx, status)
			}),
			// Spacer
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(0, gtx.Dp(unit.Dp(20)))}
			}),
			// Models section
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutModelsSection(gtx, status)
			}),
			// Spacer
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(0, gtx.Dp(unit.Dp(20)))}
			}),
			// Queue section
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutQueueSection(gtx, status)
			}),
		)
	})
}

func (a *App) layoutVRAMSection(gtx layout.Context, status arbiter.Status) layout.Dimensions {
	return layoutCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Header row: "VRAM" ... "37.0 / 100 GB"
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Baseline, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body2(a.theme, "VRAM")
						l.Color = textSecondary
						l.Font.Weight = font.Medium
						l.TextSize = unit.Sp(11)
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Body1(a.theme, fmt.Sprintf("%.1f", status.VRAMUsedGB))
								l.Color = textBright
								l.Font.Weight = font.Bold
								l.TextSize = unit.Sp(22)
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									l := material.Body2(a.theme, fmt.Sprintf("/ %.0f GB", status.VRAMBudgetGB))
									l.Color = textMuted
									l.TextSize = unit.Sp(13)
									return l.Layout(gtx)
								})
							}),
						)
					}),
				)
			}),
			// Gauge bar
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.layoutGaugeBar(gtx, status.VRAMUsedGB, status.VRAMBudgetGB)
				})
			}),
		)
	})
}

func (a *App) layoutGaugeBar(gtx layout.Context, used, budget float64) layout.Dimensions {
	barH := gtx.Dp(unit.Dp(8))
	barW := gtx.Constraints.Max.X
	radius := barH / 2

	// Track background
	trackRect := clip.RRect{
		Rect: image.Rect(0, 0, barW, barH),
		NE:   radius, NW: radius, SE: radius, SW: radius,
	}
	paint.FillShape(gtx.Ops, gaugeTrack, trackRect.Op(gtx.Ops))

	// Fill
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

		// Glow effect — semi-transparent overlay
		glowColor := fillColor
		glowColor.A = 0x30
		glowRect := clip.RRect{
			Rect: image.Rect(0, -2, fillW, barH+2),
			NE:   radius + 1, NW: radius + 1, SE: radius + 1, SW: radius + 1,
		}
		paint.FillShape(gtx.Ops, glowColor, glowRect.Op(gtx.Ops))
	}

	return layout.Dimensions{Size: image.Pt(barW, barH)}
}

func (a *App) layoutModelsSection(gtx layout.Context, status arbiter.Status) layout.Dimensions {
	if len(status.Models) == 0 {
		return layout.Dimensions{}
	}

	// Section label
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body2(a.theme, "MODELS")
			l.Color = textMuted
			l.Font.Weight = font.Medium
			l.TextSize = unit.Sp(11)
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(0, gtx.Dp(unit.Dp(8)))}
		}),
	}

	for i := range status.Models {
		m := status.Models[i]
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutModelCard(gtx, m)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(0, gtx.Dp(unit.Dp(6)))}
			}),
		)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutModelCard(gtx layout.Context, m arbiter.Model) layout.Dimensions {
	return layoutCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Top row: status dot + name + memory
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							// Status dot
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dotSize := gtx.Dp(unit.Dp(8))
								dotColor := dotUnloaded
								if m.State == "loaded" {
									dotColor = dotLoaded
								}
								r := clip.Ellipse{Max: image.Pt(dotSize, dotSize)}.Op(gtx.Ops)
								paint.FillShape(gtx.Ops, dotColor, r)

								// Glow for loaded models
								if m.State == "loaded" {
									glowColor := dotColor
									glowColor.A = 0x40
									glowR := clip.Ellipse{
										Min: image.Pt(-2, -2),
										Max: image.Pt(dotSize+2, dotSize+2),
									}.Op(gtx.Ops)
									paint.FillShape(gtx.Ops, glowColor, glowR)
								}

								return layout.Dimensions{Size: image.Pt(dotSize, dotSize)}
							}),
							// Name
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									nameColor := textPrimary
									if m.State != "loaded" {
										nameColor = textSecondary
									}
									l := material.Body1(a.theme, m.ID)
									l.Color = nameColor
									l.Font.Weight = font.Medium
									l.TextSize = unit.Sp(14)
									return l.Layout(gtx)
								})
							}),
						)
					}),
					// Memory badge
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := fmt.Sprintf("%.0f GB", m.MemoryGB)
						return layoutBadge(gtx, a.theme, label, accentDimCyan, accentCyan)
					}),
				)
			}),
			// Bottom row: state label + job counts
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						// State label
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							stateColor := textMuted
							if m.State == "loaded" {
								stateColor = accentGreen
							}
							l := material.Body2(a.theme, m.State)
							l.Color = stateColor
							l.TextSize = unit.Sp(11)
							return l.Layout(gtx)
						}),
						// Separator
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Body2(a.theme, "|")
								l.Color = borderColor
								l.TextSize = unit.Sp(11)
								return l.Layout(gtx)
							})
						}),
						// Active jobs
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							activeColor := textMuted
							if m.ActiveJobs > 0 {
								activeColor = accentOrange
							}
							l := material.Body2(a.theme, fmt.Sprintf("%d active", m.ActiveJobs))
							l.Color = activeColor
							l.TextSize = unit.Sp(11)
							return l.Layout(gtx)
						}),
						// Separator
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Body2(a.theme, "|")
								l.Color = borderColor
								l.TextSize = unit.Sp(11)
								return l.Layout(gtx)
							})
						}),
						// Queued jobs
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							queuedColor := textMuted
							if m.QueuedJobs > 0 {
								queuedColor = accentPurple
							}
							l := material.Body2(a.theme, fmt.Sprintf("%d queued", m.QueuedJobs))
							l.Color = queuedColor
							l.TextSize = unit.Sp(11)
							return l.Layout(gtx)
						}),
						// Idle time (if applicable)
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if m.IdleSeconds == nil {
								return layout.Dimensions{}
							}
							return layout.Inset{Left: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											l := material.Body2(a.theme, "|")
											l.Color = borderColor
											l.TextSize = unit.Sp(11)
											return l.Layout(gtx)
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										l := material.Body2(a.theme, fmt.Sprintf("idle %s", formatDuration(*m.IdleSeconds)))
										l.Color = textMuted
										l.TextSize = unit.Sp(11)
										return l.Layout(gtx)
									}),
								)
							})
						}),
					)
				})
			}),
		)
	})
}

func (a *App) layoutQueueSection(gtx layout.Context, status arbiter.Status) layout.Dimensions {
	return layoutCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Header
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(a.theme, "QUEUE")
				l.Color = textSecondary
				l.Font.Weight = font.Medium
				l.TextSize = unit.Sp(11)
				return l.Layout(gtx)
			}),
			// Stats row
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Spacing: layout.SpaceEvenly}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.layoutQueueStat(gtx, "Running", status.Queue.Running, accentGreen)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.layoutQueueStat(gtx, "Queued", status.Queue.Queued, accentPurple)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.layoutQueueStat(gtx, "Done", status.Queue.Completed, accentCyan)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.layoutQueueStat(gtx, "Failed", status.Queue.Failed, accentRed)
						}),
					)
				})
			}),
		)
	})
}

func (a *App) layoutQueueStat(gtx layout.Context, label string, count int, clr color.NRGBA) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			numColor := textMuted
			if count > 0 {
				numColor = clr
			}
			l := material.Body1(a.theme, fmt.Sprintf("%d", count))
			l.Color = numColor
			l.Font.Weight = font.Bold
			l.TextSize = unit.Sp(24)
			l.Alignment = text.Middle
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(a.theme, label)
				l.Color = textMuted
				l.TextSize = unit.Sp(10)
				l.Alignment = text.Middle
				return l.Layout(gtx)
			})
		}),
	)
}

// layoutCard renders a rounded card container.
func layoutCard(gtx layout.Context, content func(layout.Context) layout.Dimensions) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			r := gtx.Dp(unit.Dp(8))
			bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Min.Y)
			// Border
			borderRR := clip.RRect{Rect: bounds, NE: r, NW: r, SE: r, SW: r}
			paint.FillShape(gtx.Ops, borderColor, borderRR.Op(gtx.Ops))
			// Inner fill
			innerBounds := image.Rect(1, 1, bounds.Max.X-1, bounds.Max.Y-1)
			innerRR := clip.RRect{Rect: innerBounds, NE: r - 1, NW: r - 1, SE: r - 1, SW: r - 1}
			paint.FillShape(gtx.Ops, cardColor, innerRR.Op(gtx.Ops))
			return layout.Dimensions{Size: bounds.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: unit.Dp(14), Bottom: unit.Dp(14),
				Left: unit.Dp(16), Right: unit.Dp(16),
			}.Layout(gtx, content)
		}),
	)
}

// layoutBadge renders a small pill-shaped badge.
func layoutBadge(gtx layout.Context, th *material.Theme, label string, bg, fg color.NRGBA) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			r := gtx.Dp(unit.Dp(4))
			bounds := image.Rect(0, 0, gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
			rr := clip.RRect{Rect: bounds, NE: r, NW: r, SE: r, SW: r}
			paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
			return layout.Dimensions{Size: bounds.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: unit.Dp(3), Bottom: unit.Dp(3),
				Left: unit.Dp(8), Right: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(th, label)
				l.Color = fg
				l.TextSize = unit.Sp(10)
				l.Font.Weight = font.Medium
				return l.Layout(gtx)
			})
		}),
	)
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
