package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alt-bot/ent"
)

type ShopItemKind string

const (
	ShopItemKindWorkReset      ShopItemKind = "work_reset"
	ShopItemKindXpPack         ShopItemKind = "xp_pack"
	ShopItemKindAltPack        ShopItemKind = "alt_pack"
	ShopItemKindSecurityCamera ShopItemKind = "security_camera"
)

// MaxSecurityCameraHoldCount caps how many security cameras a user may hold
// at once, independent of how many they buy in a single /shop purchase.
const MaxSecurityCameraHoldCount int64 = 5

type ShopItem struct {
	ID          string
	Name        string
	Description string
	Price       int64
	MaxQuantity int64
	Kind        ShopItemKind
	Value       int64
}

type ShopPurchaseResult struct {
	Item         ShopItem
	Quantity     int64
	TotalPrice   int64
	BalanceAfter int64
	XpGain       int64
	AltGain      int64
	CameraGain   int64
	WorkReset    bool
}

type ShopItemNotFoundError struct {
	ItemID string
}

func (e *ShopItemNotFoundError) Error() string {
	return fmt.Sprintf("shop item not found: %s", e.ItemID)
}

type ShopQuantityError struct {
	Requested int64
	Max       int64
}

func (e *ShopQuantityError) Error() string {
	return fmt.Sprintf("shop quantity exceeds limit: requested=%d max=%d", e.Requested, e.Max)
}

type ShopHoldingLimitError struct {
	ItemID  string
	Max     int64
	Current int64
}

func (e *ShopHoldingLimitError) Error() string {
	return fmt.Sprintf("shop holding limit exceeded: item=%s max=%d current=%d", e.ItemID, e.Max, e.Current)
}

func defaultShopCatalog() []ShopItem {
	return []ShopItem{
		{ID: "work_reset", Name: "Work Reset", Description: "Workクールダウンを即時解除します。", Price: 2500, MaxQuantity: 3, Kind: ShopItemKindWorkReset},
		{ID: "xp_pack", Name: "XP Pack", Description: "XPを追加で獲得します。", Price: 1000, MaxQuantity: 50, Kind: ShopItemKindXpPack, Value: 50},
		{ID: "alt_pack", Name: "ALT Pack", Description: "ALTokenを追加で獲得します。", Price: 5000, MaxQuantity: 20, Kind: ShopItemKindAltPack, Value: 10},
		{ID: "security_camera", Name: "防犯カメラ", Description: "/robによる強奪を1回だけ防ぎます(使用後に1個消滅)。", Price: 4000, MaxQuantity: 5, Kind: ShopItemKindSecurityCamera},
	}
}

func (s *EconomyService) ShopItems() []ShopItem {
	items := defaultShopCatalog()
	result := make([]ShopItem, len(items))
	copy(result, items)
	return result
}

func (s *EconomyService) ShopItemByID(itemID string) (ShopItem, bool) {
	normalized := strings.TrimSpace(strings.ToLower(itemID))
	for _, item := range defaultShopCatalog() {
		if item.ID == normalized {
			return item, true
		}
	}
	return ShopItem{}, false
}

func (s *EconomyService) BuyShopItem(ctx context.Context, discordID, itemID string, quantity int64) (ShopPurchaseResult, error) {
	if quantity <= 0 {
		return ShopPurchaseResult{}, fmt.Errorf("quantity must be positive")
	}

	item, ok := s.ShopItemByID(itemID)
	if !ok {
		return ShopPurchaseResult{}, &ShopItemNotFoundError{ItemID: itemID}
	}
	if item.MaxQuantity > 0 && quantity > item.MaxQuantity {
		return ShopPurchaseResult{}, &ShopQuantityError{Requested: quantity, Max: item.MaxQuantity}
	}

	totalPrice := item.Price * quantity
	if totalPrice <= 0 {
		return ShopPurchaseResult{}, fmt.Errorf("invalid shop price")
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	var result ShopPurchaseResult
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		u, err := s.lockOrCreateUserForUpdateTx(ctx, tx, discordID, "shop")
		if err != nil {
			return err
		}
		if u.Balance < totalPrice {
			return &InsufficientYenError{Need: totalPrice, Have: u.Balance}
		}

		newBalance := u.Balance - totalPrice
		newXP := u.Xp
		newALT := u.CryptoBalance
		newCameraCount := u.SecurityCameraCount
		workReset := false

		switch item.Kind {
		case ShopItemKindWorkReset:
			workReset = true
		case ShopItemKindXpPack:
			newXP += item.Value * quantity
		case ShopItemKindAltPack:
			newALT += item.Value * quantity
		case ShopItemKindSecurityCamera:
			newCameraCount += quantity
			if newCameraCount > MaxSecurityCameraHoldCount {
				return &ShopHoldingLimitError{ItemID: item.ID, Max: MaxSecurityCameraHoldCount, Current: u.SecurityCameraCount}
			}
		default:
			return fmt.Errorf("unsupported shop item kind: %s", item.Kind)
		}

		updater := tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			SetXp(newXP).
			SetCryptoBalance(newALT).
			SetSecurityCameraCount(newCameraCount)
		if workReset {
			updater = updater.SetWorkEndAt(time.Unix(0, 0).UTC())
		}
		if _, err := updater.Save(ctx); err != nil {
			return fmt.Errorf("failed to update user for shop purchase: %w", err)
		}

		_, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "shop_buy_" + item.ID,
			YenDelta:     -totalPrice,
			ALTDelta:     newALT - u.CryptoBalance,
			XPDelta:      newXP - u.Xp,
			Amount:       quantity,
			SettledPrice: 0,
			PriceAfter:   0,
			BalanceAfter: newBalance,
			ALTAfter:     newALT,
		})
		if err != nil {
			return err
		}

		result = ShopPurchaseResult{
			Item:         item,
			Quantity:     quantity,
			TotalPrice:   totalPrice,
			BalanceAfter: newBalance,
			XpGain:       newXP - u.Xp,
			AltGain:      newALT - u.CryptoBalance,
			CameraGain:   newCameraCount - u.SecurityCameraCount,
			WorkReset:    workReset,
		}
		return nil
	})
	if err != nil {
		return ShopPurchaseResult{}, err
	}

	return result, nil
}
