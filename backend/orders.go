package main

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errInsufficientStock   = errors.New("insufficient stock")
	errInsufficientBalance = errors.New("insufficient balance")
	errInvalidOrder        = errors.New("invalid order")
	errCredentialMismatch  = errors.New("credential mismatch")
	digitsOnly             = regexp.MustCompile(`^\d+$`)
)

type createOrderRequest struct {
	SKUID          uint   `json:"sku_id" binding:"required"`
	Quantity       int    `json:"quantity" binding:"required"`
	PaymentMethod  string `json:"payment_method"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
	ContactType    string `json:"contact_type"`
	Contact        string `json:"contact"`
	QueryPassword  string `json:"query_password"`
}

type orderResult struct {
	Order   Order     `json:"order"`
	Item    OrderItem `json:"item"`
	Payment *Payment  `json:"payment,omitempty"`
	Cards   []string  `json:"cards,omitempty"`
	Contact string    `json:"contact,omitempty"`
}

func (a *App) createMemberOrder(c *gin.Context) {
	var req createOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.PaymentMethod != "balance" && req.PaymentMethod != "online" {
		fail(c, http.StatusBadRequest, "invalid_payment_method", "支付方式只能是余额或在线支付")
		return
	}
	if (req.PaymentMethod == "balance" && !a.config.BalancePaymentEnabled) || (req.PaymentMethod == "online" && !a.config.OnlinePaymentEnabled) {
		fail(c, http.StatusForbidden, "payment_method_disabled", "当前支付方式未开放")
		return
	}
	user := currentUser(c)
	result, created, err := a.createOrder(&user, req)
	if err != nil {
		a.orderError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, result)
}

func (a *App) createGuestOrder(c *gin.Context) {
	if !a.config.GuestCheckoutEnabled || !a.config.OnlinePaymentEnabled {
		fail(c, http.StatusForbidden, "guest_checkout_disabled", "当前未开放游客购买")
		return
	}
	var req createOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	req.PaymentMethod = "online"
	result, created, err := a.createOrder(nil, req)
	if err != nil {
		a.orderError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, result)
}

func (a *App) orderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errInsufficientStock):
		fail(c, http.StatusConflict, "insufficient_stock", "库存不足")
	case errors.Is(err, errInsufficientBalance):
		fail(c, http.StatusConflict, "insufficient_balance", "余额不足")
	case errors.Is(err, errCredentialMismatch):
		fail(c, http.StatusForbidden, "order_mismatch", "订单信息不匹配")
	case errors.Is(err, errInvalidOrder):
		fail(c, http.StatusBadRequest, "invalid_order", "订单参数不正确")
	default:
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
	}
}

func (a *App) createOrder(user *User, req createOrderRequest) (orderResult, bool, error) {
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.SKUID == 0 || req.Quantity < 1 || req.Quantity > a.config.MaxPurchaseQuantity || len(req.IdempotencyKey) < 8 || len(req.IdempotencyKey) > 100 {
		return orderResult{}, false, errInvalidOrder
	}
	if user == nil && req.PaymentMethod != "online" {
		return orderResult{}, false, errInvalidOrder
	}
	var normalizedContact, contactCipher, contactDigest, queryHash string
	var err error
	if user == nil {
		normalizedContact, err = normalizeContact(req.ContactType, req.Contact)
		if err != nil || len(req.QueryPassword) < 6 || len(req.QueryPassword) > 32 {
			return orderResult{}, false, errInvalidOrder
		}
		contactCipher, err = encryptString(a.config.ContactEncryptKey, normalizedContact)
		if err != nil {
			return orderResult{}, false, err
		}
		contactDigest = keyedHash(a.config.ContactHashKey, req.ContactType+":"+normalizedContact)
		queryHash, err = passwordHash(req.QueryPassword)
		if err != nil {
			return orderResult{}, false, err
		}
	}

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	var existing Order
	if err := a.db.Where("idempotency_key = ?", req.IdempotencyKey).First(&existing).Error; err == nil {
		if user != nil {
			if existing.UserID == nil || *existing.UserID != user.ID {
				return orderResult{}, false, errCredentialMismatch
			}
		} else if existing.UserID != nil || !hmac.Equal([]byte(existing.ContactHash), []byte(contactDigest)) || !passwordMatches(existing.QueryPasswordHash, req.QueryPassword) {
			return orderResult{}, false, errCredentialMismatch
		}
		result, err := a.orderResult(a.db, existing, true)
		return result, false, err
	} else if !isNotFound(err) {
		return orderResult{}, false, err
	}

	var created Order
	err = a.db.Transaction(func(tx *gorm.DB) error {
		var sku SKU
		if err := tx.Preload("Product").First(&sku, req.SKUID).Error; err != nil || sku.Status != "active" || sku.Product.Status != "active" {
			return errInvalidOrder
		}
		var cards []Card
		if err := tx.Where("sku_id = ? AND status = ?", sku.ID, "available").Order("id").Limit(req.Quantity).Find(&cards).Error; err != nil {
			return err
		}
		if len(cards) != req.Quantity {
			return errInsufficientStock
		}
		now := time.Now()
		created = Order{
			OrderNo: randomID("ORD"), PaymentMethod: req.PaymentMethod, Status: "pending_payment", Currency: "CNY",
			TotalAmountCents: sku.SalePriceCents * int64(req.Quantity), IdempotencyKey: req.IdempotencyKey,
		}
		if user != nil {
			created.UserID = &user.ID
		} else {
			created.ContactType = req.ContactType
			created.ContactCiphertext = contactCipher
			created.ContactHash = contactDigest
			created.QueryPasswordHash = queryHash
		}
		if req.PaymentMethod == "balance" {
			created.Status = "completed"
			created.PaidAt = &now
			created.CompletedAt = &now
		} else {
			expires := now.Add(a.config.PaymentTimeout)
			created.ExpiresAt = &expires
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		item := OrderItem{OrderID: created.ID, SKUID: sku.ID, ProductNameSnapshot: sku.Product.Name, SKUNameSnapshot: sku.Name, UnitPriceCents: sku.SalePriceCents, Quantity: req.Quantity, SubtotalCents: created.TotalAmountCents}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}

		if req.PaymentMethod == "balance" {
			if user == nil {
				return errInvalidOrder
			}
			update := tx.Model(&User{}).Where("id = ? AND status = ? AND balance_cents >= ?", user.ID, "active", created.TotalAmountCents).
				Updates(map[string]any{"balance_cents": gorm.Expr("balance_cents - ?", created.TotalAmountCents), "version": gorm.Expr("version + 1")})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return errInsufficientBalance
			}
			var updated User
			if err := tx.First(&updated, user.ID).Error; err != nil {
				return err
			}
			ledger := BalanceLedger{UserID: user.ID, Type: "purchase", Direction: "out", AmountCents: created.TotalAmountCents, BalanceAfterCents: updated.BalanceCents, RefType: "order", RefID: created.ID, Reason: "购买商品", IdempotencyKey: req.IdempotencyKey}
			if err := tx.Create(&ledger).Error; err != nil {
				return err
			}
			for _, card := range cards {
				if err := sellCard(tx, card.ID, created.ID, now); err != nil {
					return err
				}
				if err := tx.Create(&OrderCard{OrderID: created.ID, OrderItemID: item.ID, CardID: card.ID}).Error; err != nil {
					return err
				}
			}
		} else {
			for _, card := range cards {
				update := tx.Model(&Card{}).Where("id = ? AND status = ?", card.ID, "available").Updates(map[string]any{"status": "reserved", "reserved_order_id": created.ID, "reserved_until": created.ExpiresAt})
				if update.Error != nil || update.RowsAffected != 1 {
					return errInsufficientStock
				}
				if err := tx.Create(&OrderCard{OrderID: created.ID, OrderItemID: item.ID, CardID: card.ID}).Error; err != nil {
					return err
				}
			}
			payURL := fmt.Sprintf("%s/api/v1/dev/payments/%s/pay", a.config.BaseURL, created.OrderNo)
			payment := Payment{OrderID: created.ID, Provider: a.config.PaymentProvider, MerchantOrderNo: created.OrderNo, Currency: "CNY", AmountCents: created.TotalAmountCents, Status: "created", PayURL: payURL}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}
		}
		return tx.Create(&OrderEvent{OrderID: created.ID, ToStatus: created.Status, EventType: "order_created"}).Error
	})
	if err != nil {
		return orderResult{}, false, err
	}
	result, err := a.orderResult(a.db, created, true)
	return result, true, err
}

func sellCard(tx *gorm.DB, cardID, orderID uint, now time.Time) error {
	update := tx.Model(&Card{}).Where("id = ? AND status IN ?", cardID, []string{"available", "reserved"}).Updates(map[string]any{
		"status": "sold", "sold_order_id": orderID, "sold_at": now, "reserved_order_id": nil, "reserved_until": nil,
	})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return errInsufficientStock
	}
	return nil
}

func normalizeContact(kind, value string) (string, error) {
	value = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), " ", ""), "-", "")
	switch kind {
	case "qq":
		if len(value) < 5 || len(value) > 15 || !digitsOnly.MatchString(value) {
			return "", errInvalidOrder
		}
	case "phone":
		value = strings.TrimPrefix(value, "+")
		if len(value) < 7 || len(value) > 15 || !digitsOnly.MatchString(value) {
			return "", errInvalidOrder
		}
	default:
		return "", errInvalidOrder
	}
	return value, nil
}

func (a *App) orderResult(db *gorm.DB, order Order, revealCards bool) (orderResult, error) {
	var item OrderItem
	if err := db.Where("order_id = ?", order.ID).First(&item).Error; err != nil {
		return orderResult{}, err
	}
	result := orderResult{Order: order, Item: item}
	if order.PaymentMethod == "online" {
		var payment Payment
		if err := db.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
			return orderResult{}, err
		}
		result.Payment = &payment
	}
	if revealCards && order.Status == "completed" {
		var cards []Card
		if err := db.Table("cards").Joins("JOIN order_cards ON order_cards.card_id = cards.id").Where("order_cards.order_id = ?", order.ID).Order("cards.id").Find(&cards).Error; err != nil {
			return orderResult{}, err
		}
		for _, card := range cards {
			secret, err := decryptString(a.config.CardEncryptKey, card.SecretCiphertext)
			if err != nil {
				return orderResult{}, err
			}
			result.Cards = append(result.Cards, secret)
		}
	}
	return result, nil
}

func (a *App) processPayment(orderNo, tradeNo string, amount int64, currency, eventHash string, now time.Time) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.db.Transaction(func(tx *gorm.DB) error {
		var order Order
		if err := tx.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return err
		}
		var payment Payment
		if err := tx.Where("order_id = ?", order.ID).First(&payment).Error; err != nil {
			return err
		}
		if payment.AmountCents != amount || payment.Currency != currency || order.PaymentMethod != "online" {
			return errInvalidOrder
		}
		if payment.Status == "success" {
			if payment.ProviderTradeNo != tradeNo {
				return errInvalidOrder
			}
			return nil
		}
		if payment.ProviderTradeNo != "" && payment.ProviderTradeNo != tradeNo {
			return errInvalidOrder
		}

		var item OrderItem
		if err := tx.Where("order_id = ?", order.ID).First(&item).Error; err != nil {
			return err
		}
		var cards []Card
		if order.Status == "pending_payment" {
			if err := tx.Where("reserved_order_id = ? AND status = ?", order.ID, "reserved").Order("id").Find(&cards).Error; err != nil {
				return err
			}
		}
		if len(cards) != item.Quantity {
			if err := tx.Model(&Card{}).Where("reserved_order_id = ? AND status = ?", order.ID, "reserved").Updates(map[string]any{"status": "available", "reserved_order_id": nil, "reserved_until": nil}).Error; err != nil {
				return err
			}
			cards = nil
			if err := tx.Where("sku_id = ? AND status = ?", item.SKUID, "available").Order("id").Limit(item.Quantity).Find(&cards).Error; err != nil {
				return err
			}
			if len(cards) != item.Quantity {
				if err := tx.Model(&Payment{}).Where("id = ?", payment.ID).Updates(map[string]any{"status": "success", "provider_trade_no": tradeNo, "callback_event_hash": eventHash, "paid_at": now, "callback_at": now}).Error; err != nil {
					return err
				}
				old := order.Status
				if err := tx.Model(&Order{}).Where("id = ?", order.ID).Updates(map[string]any{"status": "delivery_failed", "paid_at": now}).Error; err != nil {
					return err
				}
				return tx.Create(&OrderEvent{OrderID: order.ID, FromStatus: old, ToStatus: "delivery_failed", EventType: "payment_stock_shortage", RefID: tradeNo}).Error
			}
			if err := tx.Where("order_id = ?", order.ID).Delete(&OrderCard{}).Error; err != nil {
				return err
			}
			for _, card := range cards {
				if err := tx.Create(&OrderCard{OrderID: order.ID, OrderItemID: item.ID, CardID: card.ID}).Error; err != nil {
					return err
				}
			}
		}
		for _, card := range cards {
			if err := sellCard(tx, card.ID, order.ID, now); err != nil {
				return err
			}
		}
		if err := tx.Model(&Payment{}).Where("id = ?", payment.ID).Updates(map[string]any{"status": "success", "provider_trade_no": tradeNo, "callback_event_hash": eventHash, "paid_at": now, "callback_at": now}).Error; err != nil {
			return err
		}
		old := order.Status
		if err := tx.Model(&Order{}).Where("id = ?", order.ID).Updates(map[string]any{"status": "completed", "paid_at": now, "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Create(&OrderEvent{OrderID: order.ID, FromStatus: old, ToStatus: "completed", EventType: "payment_success", RefID: tradeNo}).Error
	})
}

func (a *App) releaseExpired(now time.Time) (int, error) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	count := 0
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var orders []Order
		if err := tx.Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", "pending_payment", now).Find(&orders).Error; err != nil {
			return err
		}
		for _, order := range orders {
			if err := tx.Model(&Card{}).Where("reserved_order_id = ? AND status = ?", order.ID, "reserved").Updates(map[string]any{"status": "available", "reserved_order_id": nil, "reserved_until": nil}).Error; err != nil {
				return err
			}
			if err := tx.Where("order_id = ?", order.ID).Delete(&OrderCard{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Order{}).Where("id = ? AND status = ?", order.ID, "pending_payment").Update("status", "expired").Error; err != nil {
				return err
			}
			if err := tx.Model(&Payment{}).Where("order_id = ? AND status = ?", order.ID, "created").Update("status", "closed").Error; err != nil {
				return err
			}
			if err := tx.Create(&OrderEvent{OrderID: order.ID, FromStatus: "pending_payment", ToStatus: "expired", EventType: "payment_expired"}).Error; err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}
