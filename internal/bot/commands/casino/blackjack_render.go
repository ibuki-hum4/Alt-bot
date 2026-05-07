package casino

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fogleman/gg"
)

func renderBlackjackStatePNG(view blackjackRenderState) ([]byte, error) {
	dc := gg.NewContext(blackjackImageWidth, blackjackImageHeight)
	if bg, err := loadBlackjackBackground(); err == nil {
		drawBlackjackCardImage(dc, bg, 0, 0, float64(blackjackImageWidth), float64(blackjackImageHeight))
	} else {
		return nil, err
	}

	if err := drawBlackjackSessionFrame(dc, view); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode blackjack png: %w", err)
	}
	return buf.Bytes(), nil
}

func drawBlackjackSessionFrame(dc *gg.Context, view blackjackRenderState) error {
	titleFace, err := casinoFontFace(48)
	if err != nil {
		return err
	}
	labelFace, err := casinoFontFace(24)
	if err != nil {
		return err
	}
	balanceFace, err := casinoFontFace(30)
	if err != nil {
		return err
	}

	dc.SetFontFace(titleFace)
	dc.SetHexColor("#d6b97c")
	dc.DrawStringAnchored("BLACKJACK", blackjackTitleX, blackjackTitleY, 0, 0.5)

	dc.SetFontFace(labelFace)
	dc.SetHexColor("#d6b97c")
	dc.DrawStringAnchored("ディーラー", blackjackDealerLabelX, blackjackDealerLabelY, 0, 0.5)
	dc.DrawStringAnchored("あなた", blackjackPlayerLabelX, blackjackPlayerLabelY, 0, 0.5)

	dealerCards := blackjackDealerCardsForView(view)
	dealerRowX := blackjackLeftMarginX
	dealerRowY := blackjackDealerRowY
	if err := drawBlackjackCardRow(dc, dealerRowX, dealerRowY, dealerCards, view.RevealDealer, true); err != nil {
		return err
	}

	playerBaseY := blackjackPlayerRowY
	for i, hand := range view.Hands {
		rowWidth := blackjackCardRowWidth(len(hand.Cards))
		rowX := float64(blackjackImageWidth) - blackjackRightMarginX - rowWidth
		rowY := playerBaseY + float64(i)*(blackjackCardH+blackjackPlayerRowGap)
		if err := drawBlackjackCardRow(dc, rowX, rowY, hand.Cards, true, false); err != nil {
			return err
		}
	}

	dc.SetFontFace(balanceFace)
	dc.SetHexColor("#d6b97c")
	dc.DrawStringAnchored(fmt.Sprintf("残高 %d ¥", view.Balance), blackjackBalanceX, blackjackBalanceY, 1, 0.5)
	return nil
}

func blackjackDealerCardsForView(view blackjackRenderState) []blackjackCard {
	cards := make([]blackjackCard, 0, len(view.Dealer))
	for _, card := range view.Dealer {
		cards = append(cards, card)
	}
	return cards
}

func drawBlackjackCardRow(dc *gg.Context, x float64, y float64, cards []blackjackCard, reveal bool, isDealer bool) error {
	for i, card := range cards {
		left := x + float64(i)*(blackjackCardW+blackjackCardGap)
		w := blackjackCardW
		h := blackjackCardH
		if isDealer && !reveal && i == 1 {
			backPath := blackjackBackAssetPath(isDealer)
			img, err := loadBlackjackCardImage(backPath)
			if err != nil {
				return err
			}
			drawBlackjackCardImage(dc, img, left, y, w, h)
			continue
		}
		imgPath := blackjackCardAssetPath(card, isDealer)
		img, err := loadBlackjackCardImage(imgPath)
		if err != nil {
			return err
		}
		drawBlackjackCardImage(dc, img, left, y, w, h)
	}
	return nil
}

var blackjackCardImageCache sync.Map
var blackjackBackgroundMu sync.Mutex
var blackjackBackgroundImage image.Image
var blackjackBackgroundModTime time.Time
var blackjackBackgroundErr error

func loadBlackjackCardImage(path string) (image.Image, error) {
	if cached, ok := blackjackCardImageCache.Load(path); ok {
		return cached.(image.Image), nil
	}
	img, err := gg.LoadImage(path)
	if err != nil {
		return nil, err
	}
	blackjackCardImageCache.Store(path, img)
	return img, nil
}

func loadBlackjackBackground() (image.Image, error) {
	info, err := os.Stat(blackjackBackgroundAssetPath)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime()

	blackjackBackgroundMu.Lock()
	defer blackjackBackgroundMu.Unlock()
	if blackjackBackgroundImage != nil && blackjackBackgroundErr == nil && modTime.Equal(blackjackBackgroundModTime) {
		return blackjackBackgroundImage, nil
	}

	img, err := gg.LoadImage(blackjackBackgroundAssetPath)
	if err != nil {
		blackjackBackgroundErr = err
		return nil, err
	}
	blackjackBackgroundImage = img
	blackjackBackgroundModTime = modTime
	blackjackBackgroundErr = nil
	return blackjackBackgroundImage, nil
}

func drawBlackjackCardImage(dc *gg.Context, img image.Image, x float64, y float64, w float64, h float64) {
	if img == nil {
		return
	}
	b := img.Bounds()
	iw := float64(b.Dx())
	ih := float64(b.Dy())
	if iw == 0 || ih == 0 {
		return
	}
	dc.Push()
	dc.Translate(x, y)
	dc.Scale(w/iw, h/ih)
	dc.DrawImage(img, 0, 0)
	dc.Pop()
}

func blackjackCardAssetPath(card blackjackCard, isDealer bool) string {
	baseDir := blackjackPlayerAssetDir
	if isDealer {
		baseDir = blackjackDealerAssetDir
	}
	if card.Value < 0 {
		return filepath.Join(baseDir, "back.png")
	}
	suits := []string{"s", "h", "d", "k"}
	suitIndex := (card.Value / 13) % 4
	if suitIndex < 0 || suitIndex >= len(suits) {
		suitIndex = 0
	}
	rank := blackjackCardRank(card) + 1
	fileName := fmt.Sprintf("%s%02d.png", suits[suitIndex], rank)
	return filepath.Join(baseDir, fileName)
}

func blackjackBackAssetPath(isDealer bool) string {
	if isDealer {
		return filepath.Join(blackjackDealerAssetDir, "back.png")
	}
	return filepath.Join(blackjackPlayerAssetDir, "back.png")
}

const (
	blackjackImageWidth  = 1672
	blackjackImageHeight = 941
	blackjackCardW      = 150.0
	blackjackCardH      = 225.0
	blackjackCardGap    = 18.0
	blackjackLeftMarginX = 140.0
	blackjackRightMarginX = 140.0
	blackjackDealerRowY = 180.0
	blackjackPlayerRowY = 560.0
	blackjackPlayerRowGap = 28.0
	blackjackTitleX = 140.0
	blackjackTitleY = 96.0
	blackjackBalanceX = 1600.0
	blackjackBalanceY = 884.0
	blackjackDealerLabelX = 140.0
	blackjackDealerLabelY = 150.0
	blackjackPlayerLabelX = 1230.0
	blackjackPlayerLabelY = 520.0
	blackjackBackgroundAssetPath = "internal/assets/black-jack-background.png"
	blackjackDealerAssetDir = "internal/assets/type-A"
	blackjackPlayerAssetDir = "internal/assets/type-B"
)

func blackjackCardRowWidth(cardCount int) float64 {
	if cardCount <= 0 {
		return blackjackCardW
	}
	return float64(cardCount)*blackjackCardW + float64(cardCount-1)*blackjackCardGap
}
