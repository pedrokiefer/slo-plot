package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

type Window struct {
	Duration time.Duration
	BurnRate float64
	Kind     string
}

type DurationTicks struct{}

// Ticks implements plot.Ticker.
func (t DurationTicks) Ticks(min, max float64) []plot.Tick {
	ticker := plot.LogTicks{}

	ticks := ticker.Ticks(min, max)
	for i := range ticks {
		tick := &ticks[i]
		if tick.Label == "" {
			continue
		}
		// In seconds
		secondsToConvert := int64(tick.Value * 60 * 60)
		hours := secondsToConvert / 3600
		secondsToConvert -= hours * 3600
		minutes := secondsToConvert / 60
		secondsToConvert -= minutes * 60
		seconds := secondsToConvert
		tick.Label = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return ticks
}

func main() {
	sloPeriod := 30.0
	slo := 99.9

	ticket, page := genSLOAlertPoints((1 - (slo / 100.0)), []Window{
		{
			Duration: time.Duration(1 * time.Hour),
			BurnRate: 14.4,
			Kind:     "page",
		},
		{
			Duration: time.Duration(6 * time.Hour),
			BurnRate: 6,
			Kind:     "page",
		},
		{
			Duration: time.Duration(24 * time.Hour),
			BurnRate: 3,
			Kind:     "ticket",
		},
		{
			Duration: time.Duration(72 * time.Hour),
			BurnRate: 1,
			Kind:     "ticket",
		},
	})

	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "Alert Notification"

	// X Axis Configs
	p.X.Scale = plot.LogScale{}
	p.X.Tick.Marker = plot.LogTicks{}
	p.X.Min = 0.09
	p.X.Max = 100

	// Y Axis Configs
	p.Y.Scale = plot.LogScale{}
	p.Y.Min = 1 / 120.0
	p.Y.Max = sloPeriod * 24.0
	p.Y.Tick.Marker = DurationTicks{}

	p.X.Label.Text = "Error Rate"
	p.Y.Label.Text = "Detection Time"

	// Plot Ticket
	l, err := plotter.NewLine(ticket)
	if err != nil {
		return
	}
	l.Color = color.RGBA{241, 90, 96, 255}
	l.Width = 0.5 * vg.Millimeter
	p.Add(l)
	p.Legend.Add("Ticket", l)

	// Plot Page
	l, err = plotter.NewLine(page)
	if err != nil {
		return
	}
	l.Color = color.RGBA{122, 195, 106, 255}
	l.Width = 0.5 * vg.Millimeter
	p.Add(l)
	p.Legend.Add("Page", l)
	p.Legend.Top = true
	p.Legend.XOffs = -5 * vg.Millimeter

	p.Add(plotter.NewGrid())

	err = SavePng(p, "slo_alerts.png")
	if err != nil {
		log.Fatal(err)
	}
}

func SavePng(p *plot.Plot, file string) error {
	c := vgimg.NewWith(
		vgimg.UseDPI(96),
		vgimg.UseWH(600, 320),
		vgimg.UseBackgroundColor(color.White),
	)

	p.Draw(draw.New(c))

	pp := vgimg.PngCanvas{c}
	o, err := os.Create(file)
	if err != nil {
		return err
	}
	pp.WriteTo(o)
	o.Close()
	return nil
}

type ThresholdData struct {
	ErrorThreshold float64
	Duration       float64
}

func detectionTimeForErrorRate(thresholds map[string][]ThresholdData, x float64) (string, float64) {
	var kind string
	var value float64
	value = 0

	for k, ths := range thresholds {
		for _, th := range ths {
			v := float64(th.ErrorThreshold / x)
			if v > th.Duration {
				continue
			}
			if value == 0 || value > v {
				value = v
				kind = k
			}
		}
	}

	return kind, value
}

func genSLOAlertPoints(errorBudget float64, windows []Window) (plotter.XYs, plotter.XYs) {
	// Number of points to plot
	n := 1000
	var ticket plotter.XYs
	var page plotter.XYs

	thresholdMap := map[string][]ThresholdData{}
	for _, w := range windows {
		d := float64(w.Duration / time.Hour)
		v := w.BurnRate * errorBudget * d
		t := ThresholdData{
			ErrorThreshold: v,
			Duration:       d,
		}
		thresholdPerWindow := thresholdMap[w.Kind]
		thresholdPerWindow = append(thresholdPerWindow, t)
		thresholdMap[w.Kind] = thresholdPerWindow
	}

	for i := 0; i < n; i++ {
		x := math.Pow(0.995, float64((n-i-1)*2))
		kind, value := detectionTimeForErrorRate(thresholdMap, x)

		p := plotter.XY{}
		p.X = x * 100
		p.Y = value

		if kind == "ticket" {
			ticket = append(ticket, p)
		} else if kind == "page" {
			page = append(page, p)
		}
	}

	return ticket, page
}
