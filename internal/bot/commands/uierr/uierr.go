// Package uierr converts internal/service error types into the Japanese
// messages shown to Discord users, so every command formats the same
// service error the same way instead of re-implementing the switch locally.
package uierr

import (
	"errors"
	"fmt"

	"alt-bot/internal/service"
)

// Format converts a known service error into a user-facing Japanese message.
// profitCapVerb controls the wording used for service.ProfitCapError
// (e.g. "獲得" for games/work, "受取" for pay), since the same error type is
// reused in contexts where "earned" and "received" read differently.
// recognized is false when err is nil or not one of the known service error
// types; callers should fall back to a generic message and log the error.
func Format(err error, profitCapVerb string) (message string, recognized bool) {
	if err == nil {
		return "", false
	}

	var insufficientYen *service.InsufficientYenError
	if errors.As(err, &insufficientYen) {
		return fmt.Sprintf("Yen不足です。必要: %d %s / 現在: %d %s",
			insufficientYen.Need, service.CurrencyYenUnit, insufficientYen.Have, service.CurrencyYenUnit), true
	}

	var insufficientALT *service.InsufficientALTError
	if errors.As(err, &insufficientALT) {
		return fmt.Sprintf("ALToken不足です。必要: %d %s / 現在: %d %s",
			insufficientALT.Need, service.CurrencyALTUnit, insufficientALT.Have, service.CurrencyALTUnit), true
	}

	var profitErr *service.ProfitCapError
	if errors.As(err, &profitErr) {
		period := "今週の"
		if profitErr.Window == "daily" {
			period = "本日の"
		}
		title := period + profitCapVerb + "上限"
		desc := fmt.Sprintf("%sに達しました。上限: %d / 既%s: %d", title, profitErr.Cap, profitCapVerb, profitErr.Earned)
		return title + ": " + desc, true
	}

	var haltedErr *service.MarketHaltedError
	if errors.As(err, &haltedErr) {
		return "市場は緊急停止中です。30分安定後に自動解除されます。", true
	}

	var circuitErr *service.CircuitLimitError
	if errors.As(err, &circuitErr) {
		return fmt.Sprintf("サーキット制限により注文上限を超えました。現在の上限: %d ALT", circuitErr.MaxQty), true
	}

	var issuanceErr *service.DailyIssuanceCapError
	if errors.As(err, &issuanceErr) {
		return fmt.Sprintf("本日の発行上限に達しました。上限: %d / 既発行: %d", issuanceErr.Cap, issuanceErr.Issued), true
	}

	var shopNotFound *service.ShopItemNotFoundError
	if errors.As(err, &shopNotFound) {
		return fmt.Sprintf("商品 %s は見つかりません。/shop list で確認してください。", shopNotFound.ItemID), true
	}

	var shopQty *service.ShopQuantityError
	if errors.As(err, &shopQty) {
		return fmt.Sprintf("購入上限を超えています。上限: %d / 指定: %d", shopQty.Max, shopQty.Requested), true
	}

	return "", false
}
