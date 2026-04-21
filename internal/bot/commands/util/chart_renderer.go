package util

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"time"

	"alt-bot/internal/service"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

const (
	chartWidth  = 1280
	chartHeight = 720
	chartMargin = 56.0
)

var chartJST = time.FixedZone("JST", 9*60*60)

func formatJSTClock(t time.Time) string {
	return t.In(chartJST).Format("15:04")
}

type ChartEventMarker struct {
	Time  string
	Event string
}

func RenderMarketChartPNG(points []service.PricePoint, snapshot service.RateSnapshot, markers []ChartEventMarker) ([]byte, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("price history is empty")
	}

	dc := gg.NewContext(chartWidth, chartHeight)
	dc.SetHexColor("#0d1117")
	dc.Clear()

	titleFace, err := loadFontFace(34)
	if err != nil {
		return nil, err
	}
	hudFace, err := loadFontFace(22)
	if err != nil {
		return nil, err
	}
	axisFace, err := loadFontFace(16)
	if err != nil {
		return nil, err
	}
	markerFace, err := loadFontFace(14)
	if err != nil {
		return nil, err
	}

	hudBottom := chartMargin + 96
	plotLeft := chartMargin
	plotRight := float64(chartWidth) - chartMargin
	plotTop := hudBottom + 12
	plotBottom := float64(chartHeight) - chartMargin

	prices := make([]float64, len(points))
	for i := range points {
		prices[i] = points[i].Price
	}
	minPrice, maxPrice := minMax(prices)
	if math.Abs(maxPrice-minPrice) < 1e-9 {
		pad := maxPrice * 0.01
		if pad < 1 {
			pad = 1
		}
		minPrice -= pad
		maxPrice += pad
	} else {
		pad := (maxPrice - minPrice) * 0.1
		minPrice -= pad
		maxPrice += pad
	}

	drawGrid(dc, plotLeft, plotTop, plotRight, plotBottom)
	drawAxesLabels(dc, axisFace, points, minPrice, maxPrice, plotLeft, plotTop, plotRight, plotBottom)
	drawEventMarkers(dc, markerFace, points, markers, plotLeft, plotTop, plotRight, plotBottom)

	lineColor := trendColor(prices[0], prices[len(prices)-1])
	coords := drawPriceLine(dc, points, minPrice, maxPrice, plotLeft, plotTop, plotRight, plotBottom, lineColor)
	drawShockPoints(dc, points, coords)
	drawHUD(dc, titleFace, hudFace, snapshot)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func loadFontFace(size float64) (font.Face, error) {
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 72})
	if err != nil {
		return nil, fmt.Errorf("failed to create font face: %w", err)
	}
	return face, nil
}

func drawGrid(dc *gg.Context, left float64, top float64, right float64, bottom float64) {
	minorX := 12
	majorX := 6
	minorY := 10
	majorY := 5

	dc.SetHexColor("#222222")
	for i := 0; i <= minorX; i++ {
		x := left + (right-left)*float64(i)/float64(minorX)
		dc.SetLineWidth(1)
		dc.MoveTo(x, top)
		dc.LineTo(x, bottom)
		dc.Stroke()
	}
	for i := 0; i <= minorY; i++ {
		y := top + (bottom-top)*float64(i)/float64(minorY)
		dc.SetLineWidth(1)
		dc.MoveTo(left, y)
		dc.LineTo(right, y)
		dc.Stroke()
	}

	dc.SetRGBA255(80, 80, 80, 255)
	for i := 0; i <= majorX; i++ {
		x := left + (right-left)*float64(i)/float64(majorX)
		dc.SetLineWidth(1.4)
		dc.MoveTo(x, top)
		dc.LineTo(x, bottom)
		dc.Stroke()
	}
	for i := 0; i <= majorY; i++ {
		y := top + (bottom-top)*float64(i)/float64(majorY)
		dc.SetLineWidth(1.4)
		dc.MoveTo(left, y)
		dc.LineTo(right, y)
		dc.Stroke()
	}
}

func drawAxesLabels(dc *gg.Context, face font.Face, points []service.PricePoint, minPrice float64, maxPrice float64, left float64, top float64, right float64, bottom float64) {
	dc.SetFontFace(face)
	dc.SetHexColor("#aaaaaa")

	yTicks := 6
	for i := 0; i <= yTicks; i++ {
		v := maxPrice - (maxPrice-minPrice)*float64(i)/float64(yTicks)
		y := top + (bottom-top)*float64(i)/float64(yTicks)
		dc.DrawStringAnchored(fmt.Sprintf("%.2f", v), left-8, y, 1, 0.5)
	}

	xTicks := 6
	for i := 0; i <= xTicks; i++ {
		idx := int(float64(len(points)-1) * float64(i) / float64(xTicks))
		x := left + (right-left)*float64(i)/float64(xTicks)
		dc.DrawStringAnchored(formatJSTClock(points[idx].CreatedAt), x, bottom+22, 0.5, 0.5)
	}
}

func drawEventMarkers(dc *gg.Context, face font.Face, points []service.PricePoint, markers []ChartEventMarker, left float64, top float64, right float64, bottom float64) {
	if len(markers) == 0 {
		return
	}
	indexByTime := make(map[string]int, len(points))
	for i := range points {
		indexByTime[formatJSTClock(points[i].CreatedAt)] = i
	}
	dc.SetFontFace(face)
	dc.SetHexColor("#ffaa00")
	for _, m := range markers {
		idx, ok := indexByTime[m.Time]
		if !ok {
			continue
		}
		x := pointX(idx, len(points), left, right)
		dc.SetLineWidth(1.2)
		dc.MoveTo(x, top)
		dc.LineTo(x, bottom)
		dc.Stroke()
		dc.DrawStringAnchored(m.Event, x+4, top+14, 0, 0.5)
	}
}

func drawPriceLine(dc *gg.Context, points []service.PricePoint, minPrice float64, maxPrice float64, left float64, top float64, right float64, bottom float64, lineColor string) [][2]float64 {
	coords := make([][2]float64, len(points))
	for i := range points {
		x := pointX(i, len(points), left, right)
		y := pointY(points[i].Price, minPrice, maxPrice, top, bottom)
		coords[i] = [2]float64{x, y}
	}

	dc.SetHexColor(lineColor)
	dc.SetLineWidth(2)
	dc.MoveTo(coords[0][0], coords[0][1])
	for i := 1; i < len(coords); i++ {
		dc.LineTo(coords[i][0], coords[i][1])
	}
	dc.Stroke()

	return coords
}

func drawShockPoints(dc *gg.Context, points []service.PricePoint, coords [][2]float64) {
	dc.SetHexColor("#ffffff")
	for i := 1; i < len(points); i++ {
		prev := points[i-1].Price
		curr := points[i].Price
		if prev == 0 {
			continue
		}
		change := math.Abs((curr - prev) / prev)
		if change >= 0.10 {
			dc.DrawCircle(coords[i][0], coords[i][1], 4)
			dc.Fill()
		}
	}
}

func drawHUD(dc *gg.Context, titleFace font.Face, hudFace font.Face, snapshot service.RateSnapshot) {
	dc.SetFontFace(titleFace)
	dc.SetHexColor("#f0f6fc")
	dc.DrawStringAnchored("ALToken Market Chart", chartMargin, chartMargin+20, 0, 0.5)

	dc.SetFontFace(hudFace)
	dc.SetHexColor("#c9d1d9")
	dc.DrawStringAnchored(fmt.Sprintf("現在価格: %.2f Yen", snapshot.CurrentPrice), chartMargin, chartMargin+56, 0, 0.5)

	change := "N/A"
	if snapshot.Has24h {
		change = fmt.Sprintf("%+.2f%%", snapshot.Change24h)
	}
	if snapshot.Change24h >= 0 {
		dc.SetHexColor("#00ff88")
	} else {
		dc.SetHexColor("#ff4d4f")
	}
	if !snapshot.Has24h {
		dc.SetHexColor("#aaaaaa")
	}
	dc.DrawStringAnchored(fmt.Sprintf("24h変動率: %s", change), chartMargin+360, chartMargin+56, 0, 0.5)

	eventName := "NONE"
	if snapshot.CurrentEvent != "" {
		eventName = string(snapshot.CurrentEvent)
	}
	dc.SetHexColor("#c9d1d9")
	dc.DrawStringAnchored(fmt.Sprintf("現在イベント: %s", eventName), chartMargin+700, chartMargin+56, 0, 0.5)
}

func trendColor(first float64, last float64) string {
	if first == 0 {
		return "#aaaaaa"
	}
	delta := (last - first) / first
	if delta > 0.0001 {
		return "#00ff88"
	}
	if delta < -0.0001 {
		return "#ff4d4f"
	}
	return "#aaaaaa"
}

func pointX(i int, n int, left float64, right float64) float64 {
	if n <= 1 {
		return left
	}
	return left + (right-left)*float64(i)/float64(n-1)
}

func pointY(price float64, minPrice float64, maxPrice float64, top float64, bottom float64) float64 {
	if maxPrice <= minPrice {
		return (top + bottom) / 2
	}
	ratio := (price - minPrice) / (maxPrice - minPrice)
	return bottom - ratio*(bottom-top)
}

func minMax(values []float64) (float64, float64) {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[0], sorted[len(sorted)-1]
}
