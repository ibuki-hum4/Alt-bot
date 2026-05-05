package casino

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	"alt-bot/internal/service"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	casinoImageWidth  = 1200
	casinoImageHeight = 675
)

func renderCasinoResultPNG(game string, res service.CasinoPlayResult) ([]byte, error) {
	dc := gg.NewContext(casinoImageWidth, casinoImageHeight)
	bg := backgroundColor(game)
	dc.SetHexColor(bg)
	dc.Clear()

	if err := drawCasinoFrame(dc, game, res); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode casino png: %w", err)
	}
	return buf.Bytes(), nil
}

func drawCasinoFrame(dc *gg.Context, game string, res service.CasinoPlayResult) error {
	titleFace, err := casinoFontFace(40)
	if err != nil {
		return err
	}
	hugeFace, err := casinoFontFace(72)
	if err != nil {
		return err
	}
	bodyFace, err := casinoFontFace(26)
	if err != nil {
		return err
	}
	smallFace, err := casinoFontFace(18)
	if err != nil {
		return err
	}

	drawGlowCard(dc, 48, 44, float64(casinoImageWidth-96), float64(casinoImageHeight-88), 28, "#111827", 210)

	dc.SetFontFace(titleFace)
	dc.SetHexColor("#f8fafc")
	dc.DrawStringAnchored(strings.ToUpper(game), 72, 96, 0, 0.5)

	resultText, resultColor := casinoOutcomeLabel(res.NetYen)
	dc.SetFontFace(hugeFace)
	dc.SetHexColor(resultColor)
	dc.DrawStringAnchored(resultText, 72, 178, 0, 0.5)

	dc.SetFontFace(bodyFace)
	dc.SetHexColor("#d1d5db")
	dc.DrawStringAnchored(fmt.Sprintf("Bet: %d %s", res.BetYen, service.CurrencyYenUnit), 72, 250, 0, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Payout: %d %s", res.PayoutYen, service.CurrencyYenUnit), 72, 292, 0, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Net: %+d %s", res.NetYen, service.CurrencyYenUnit), 72, 334, 0, 0.5)
	dc.DrawStringAnchored(fmt.Sprintf("Balance: %d %s", res.YenBalance, service.CurrencyYenUnit), 72, 376, 0, 0.5)

	dc.SetFontFace(smallFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored("Result Detail", 72, 448, 0, 0.5)

	if game == "blackjack" {
		drawResultPills(dc, 72, 472, res.Symbols, "#1f2937", "#38bdf8")
		drawBlackjackPanel(dc, res)
	} else {
		drawResultPills(dc, 72, 472, res.Symbols, "#1f2937", "#f97316")
		drawChinchiroPanel(dc, res)
	}

	dc.SetFontFace(smallFace)
	dc.SetHexColor("#64748b")
	dc.DrawStringAnchored("GUI casino result preview", float64(casinoImageWidth-72), float64(casinoImageHeight-54), 1, 0.5)
	return nil
}

func drawBlackjackPanel(dc *gg.Context, res service.CasinoPlayResult) {
	x := 640.0
	y := 126.0
	w := 480.0
	h := 418.0
	drawGlowCard(dc, x, y, w, h, 24, "#0f172a", 160)

	labelFace, _ := casinoFontFace(20)
	valueFace, _ := casinoFontFace(28)
	playerHand, dealerHand := blackjackHands(res)

	dc.SetFontFace(labelFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored("あなたの手札", x+24, y+46, 0, 0.5)
	dc.SetFontFace(valueFace)
	dc.SetHexColor("#f8fafc")
	dc.DrawStringAnchored(playerHand, x+24, y+82, 0, 0.5)

	dc.SetFontFace(labelFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored("ディーラーの手札", x+24, y+134, 0, 0.5)
	dc.SetFontFace(valueFace)
	dc.SetHexColor("#f8fafc")
	dc.DrawStringAnchored(dealerHand, x+24, y+170, 0, 0.5)

	dc.SetFontFace(labelFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored("ベット", x+24, y+228, 0, 0.5)
	dc.SetFontFace(valueFace)
	dc.SetHexColor("#e2e8f0")
	dc.DrawStringAnchored(fmt.Sprintf("%d yen", res.BetYen), x+24, y+262, 0, 0.5)

	dc.SetFontFace(labelFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored("残高", x+24, y+314, 0, 0.5)
	dc.SetFontFace(valueFace)
	dc.SetHexColor("#e2e8f0")
	dc.DrawStringAnchored(fmt.Sprintf("%d yen", res.YenBalance), x+24, y+348, 0, 0.5)

	drawStatLine(dc, x+24, y+392, "倍率", fmt.Sprintf("%.2fx", res.Multiplier), "#60a5fa")
}

func drawChinchiroPanel(dc *gg.Context, res service.CasinoPlayResult) {
	x := 640.0
	y := 126.0
	w := 480.0
	h := 418.0
	drawGlowCard(dc, x, y, w, h, 24, "#111827", 160)

	labelFace, _ := casinoFontFace(22)
	valueFace, _ := casinoFontFace(44)
	faceFace, _ := casinoFontFace(36)

	dc.SetFontFace(labelFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored("あなたの出目", x+24, y+38, 0, 0.5)

	faces := []string{"?", "?", "?"}
	if len(res.Symbols) > 0 {
		faces = chinchiroFaces(res.Symbols)
	}
	drawDiceRow(dc, x+18, y+78, faces, faceFace)

	dc.SetFontFace(valueFace)
	dc.SetHexColor("#f8fafc")
	resultLabel := strings.Join(res.Symbols, " / ")
	if resultLabel == "" {
		resultLabel = "-"
	}
	dc.DrawStringAnchored(resultLabel, x+24, y+228, 0, 0.5)

	drawStatLine(dc, x+24, y+290, "Multiplier", fmt.Sprintf("%.2fx", res.Multiplier), "#f59e0b")
	drawStatLine(dc, x+24, y+342, "Payout", fmt.Sprintf("%d %s", res.PayoutYen, service.CurrencyYenUnit), "#34d399")
	drawStatLine(dc, x+24, y+394, "Net", fmt.Sprintf("%+d %s", res.NetYen, service.CurrencyYenUnit), "#f87171")
}

func drawResultPills(dc *gg.Context, x float64, y float64, values []string, bgColor string, accent string) {
	pillFace, _ := casinoFontFace(18)
	cursorX := x
	for _, value := range values {
		width := math.Max(82, float64(len(value))*12+34)
		dc.SetHexColor(bgColor)
		dc.DrawRoundedRectangle(cursorX, y, width, 34, 16)
		dc.Fill()
		dc.SetHexColor(accent)
		dc.SetLineWidth(2)
		dc.DrawRoundedRectangle(cursorX, y, width, 34, 16)
		dc.Stroke()
		dc.SetFontFace(pillFace)
		dc.SetHexColor("#f8fafc")
		dc.DrawStringAnchored(value, cursorX+width/2, y+17, 0.5, 0.5)
		cursorX += width + 12
	}
}

func drawDiceRow(dc *gg.Context, x float64, y float64, faces []string, faceFace font.Face) {
	for i, face := range faces {
		left := x + float64(i)*140
		dc.SetHexColor("#f8fafc")
		dc.DrawRoundedRectangle(left, y, 112, 112, 24)
		dc.Fill()
		dc.SetHexColor("#cbd5e1")
		dc.SetLineWidth(3)
		dc.DrawRoundedRectangle(left, y, 112, 112, 24)
		dc.Stroke()
		dc.SetFontFace(faceFace)
		dc.SetHexColor("#111827")
		dc.DrawStringAnchored(face, left+56, y+60, 0.5, 0.5)
	}
}

func drawStatLine(dc *gg.Context, x float64, y float64, label string, value string, accent string) {
	labelFace, _ := casinoFontFace(20)
	valueFace, _ := casinoFontFace(28)
	dc.SetFontFace(labelFace)
	dc.SetHexColor("#94a3b8")
	dc.DrawStringAnchored(label, x, y, 0, 0.5)
	dc.SetFontFace(valueFace)
	dc.SetHexColor(accent)
	dc.DrawStringAnchored(value, x+170, y, 0, 0.5)
}

func drawGlowCard(dc *gg.Context, x float64, y float64, w float64, h float64, radius float64, color string, alpha uint8) {
	dc.SetRGBA255(0, 0, 0, 120)
	dc.DrawRoundedRectangle(x+8, y+10, w, h, radius)
	dc.Fill()
	dc.SetRGBA255(255, 255, 255, int(alpha))
	dc.DrawRoundedRectangle(x, y, w, h, radius)
	dc.Fill()
	dc.SetHexColor(color)
	dc.DrawRoundedRectangle(x, y, w, h, radius)
	dc.Stroke()
}

func casinoOutcomeLabel(net int64) (string, string) {
	switch {
	case net > 0:
		return "WIN", "#22c55e"
	case net < 0:
		return "LOSE", "#ef4444"
	default:
		return "PUSH", "#f59e0b"
	}
}

func chinchiroFaces(symbols []string) []string {
	faces := make([]string, 3)
	if len(symbols) > 0 && len(symbols[0]) == 3 {
		candidate := symbols[0]
		ok := true
		for _, r := range candidate {
			if r < '0' || r > '9' {
				ok = false
				break
			}
		}
		if ok {
			for i, r := range candidate {
				faces[i] = string(r)
			}
			return faces
		}
	}
	for i := range faces {
		if i < len(symbols) {
			faces[i] = strings.ToUpper(symbols[i])
		} else {
			faces[i] = "?"
		}
	}
	return faces
}

func blackjackHands(res service.CasinoPlayResult) (string, string) {
	key := ""
	if len(res.Symbols) > 0 {
		key = strings.ToUpper(res.Symbols[0])
	}
	switch key {
	case "BLACKJACK":
		return "A♠, K♦ (21)", "9♥, ??"
	case "WIN":
		return "6♣, J♠ (16)", "7♥, ??"
	case "PUSH":
		return "10♦, 9♣ (19)", "9♥, ??"
	case "LOSE":
		return "8♠, 7♣ (15)", "10♥, ??"
	default:
		if res.NetYen > 0 {
			return "6♣, J♠ (16)", "7♥, ??"
		}
		if res.NetYen < 0 {
			return "8♠, 7♣ (15)", "10♥, ??"
		}
		return "10♦, 9♣ (19)", "9♥, ??"
	}
}

func backgroundColor(game string) string {
	switch game {
	case "blackjack":
		return "#063b2f"
	case "chinchiro":
		return "#111827"
	default:
		return "#0b1020"
	}
}

func casinoFontFace(size float64) (font.Face, error) {
	ft, err := casinoFont()
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 72})
	if err != nil {
		return nil, fmt.Errorf("failed to create font face: %w", err)
	}
	return face, nil
}

const casinoFontPath = "internal/assets/Kiwi_Maru/KiwiMaru-Regular.ttf"

var (
	casinoFontOnce sync.Once
	casinoFontValue *opentype.Font
	casinoFontErr error
)

func casinoFont() (*opentype.Font, error) {
	casinoFontOnce.Do(func() {
		data, err := os.ReadFile(casinoFontPath)
		if err != nil {
			casinoFontErr = fmt.Errorf("failed to read font: %w", err)
			return
		}
		ft, err := opentype.Parse(data)
		if err != nil {
			casinoFontErr = fmt.Errorf("failed to parse font: %w", err)
			return
		}
		casinoFontValue = ft
	})
	if casinoFontErr != nil {
		return nil, casinoFontErr
	}
	return casinoFontValue, nil
}
