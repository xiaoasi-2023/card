package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) login(c *gin.Context) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	var user User
	login := strings.ToLower(strings.TrimSpace(req.Login))
	if err := a.db.Where("LOWER(email) = ? OR LOWER(username) = ?", login, login).First(&user).Error; err != nil || user.Status != "active" || !passwordMatches(user.PasswordHash, req.Password) {
		fail(c, http.StatusUnauthorized, "invalid_credentials", "账号或密码错误")
		return
	}
	token, err := issueToken(a.config.JWTSecret, user)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (a *App) getMe(c *gin.Context) { c.JSON(http.StatusOK, currentUser(c)) }

func (a *App) changePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	user := currentUser(c)
	if !passwordMatches(user.PasswordHash, req.CurrentPassword) || len(req.NewPassword) < 8 || len(req.NewPassword) > 72 {
		fail(c, http.StatusBadRequest, "invalid_password", "当前密码错误或新密码不符合要求")
		return
	}
	hash, _ := passwordHash(req.NewPassword)
	if err := a.db.Model(&User{}).Where("id = ?", user.ID).Update("password_hash", hash).Error; err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "修改失败")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) listMyLedgers(c *gin.Context) {
	user := currentUser(c)
	var rows []BalanceLedger
	if err := paginate(c, a.db.Where("user_id = ?", user.ID).Order("id DESC"), &rows); err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (a *App) listMyOrders(c *gin.Context) {
	user := currentUser(c)
	var rows []Order
	if err := paginate(c, a.db.Where("user_id = ?", user.ID).Order("id DESC"), &rows); err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	items := make([]orderResult, 0, len(rows))
	for _, row := range rows {
		result, err := a.orderResult(a.db, row, false)
		if err != nil {
			fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
			return
		}
		items = append(items, result)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) getMyOrder(c *gin.Context) {
	user := currentUser(c)
	var order Order
	if err := a.db.Where("order_no = ? AND user_id = ?", c.Param("order_no"), user.ID).First(&order).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "订单不存在")
		return
	}
	result, err := a.orderResult(a.db, order, true)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) queryGuestOrder(c *gin.Context) {
	var req struct {
		OrderNo       string `json:"order_no"`
		ContactType   string `json:"contact_type"`
		Contact       string `json:"contact"`
		QueryPassword string `json:"query_password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	normalized, err := normalizeContact(req.ContactType, req.Contact)
	if err != nil {
		fail(c, http.StatusForbidden, "order_mismatch", "订单信息不匹配")
		return
	}
	digest := keyedHash(a.config.ContactHashKey, req.ContactType+":"+normalized)
	var order Order
	if err := a.db.Where("order_no = ? AND user_id IS NULL AND contact_hash = ?", strings.TrimSpace(req.OrderNo), digest).First(&order).Error; err != nil || !passwordMatches(order.QueryPasswordHash, req.QueryPassword) {
		fail(c, http.StatusForbidden, "order_mismatch", "订单信息不匹配")
		return
	}
	result, err := a.orderResult(a.db, order, true)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) listPublicPlatforms(c *gin.Context) {
	var rows []Platform
	if err := a.db.Where("status = ?", "active").Order("sort DESC, id").Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

type skuPublic struct {
	SKU
	Stock *int64 `json:"stock,omitempty"`
}

func (a *App) listPublicProducts(c *gin.Context) {
	query := a.db.Preload("Platform").Where("products.status = ?", "active").Order("products.sort DESC, products.id")
	if platformID := c.Query("platform_id"); platformID != "" {
		query = query.Where("products.platform_id = ?", platformID)
	}
	var products []Product
	if err := paginate(c, query, &products); err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": products})
}

func (a *App) getPublicProduct(c *gin.Context) {
	var product Product
	if err := a.db.Preload("Platform").Where("slug = ? AND status = ?", c.Param("slug"), "active").First(&product).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "商品不存在")
		return
	}
	var skus []SKU
	if err := a.db.Where("product_id = ? AND status = ?", product.ID, "active").Find(&skus).Error; err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	result := make([]skuPublic, 0, len(skus))
	for _, sku := range skus {
		row := skuPublic{SKU: sku}
		if a.config.ShowStockCount {
			var count int64
			_ = a.db.Model(&Card{}).Where("sku_id = ? AND status = ?", sku.ID, "available").Count(&count).Error
			row.Stock = &count
		}
		result = append(result, row)
	}
	c.JSON(http.StatusOK, gin.H{"product": product, "skus": result})
}

type paymentCallbackRequest struct {
	MerchantID  string `json:"merchant_id"`
	OrderNo     string `json:"order_no"`
	TradeNo     string `json:"trade_no"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	Timestamp   int64  `json:"timestamp"`
}

func (a *App) paymentWebhook(c *gin.Context) {
	if c.Param("provider") != a.config.PaymentProvider {
		fail(c, http.StatusNotFound, "provider_not_found", "支付通道不存在")
		return
	}
	var req paymentCallbackRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.MerchantID != a.config.PaymentMerchantID || req.OrderNo == "" || req.TradeNo == "" || req.Currency != "CNY" || req.AmountCents <= 0 || time.Since(time.Unix(req.Timestamp, 0)) > 10*time.Minute || time.Until(time.Unix(req.Timestamp, 0)) > time.Minute {
		fail(c, http.StatusBadRequest, "invalid_callback", "支付回调参数不正确")
		return
	}
	expected := signPayment(a.config.PaymentMerchantKey, req)
	if !verifySignature(expected, c.GetHeader("X-Payment-Signature")) {
		fail(c, http.StatusUnauthorized, "invalid_signature", "支付签名无效")
		return
	}
	eventSum := sha256.Sum256([]byte(req.OrderNo + "|" + req.TradeNo))
	if err := a.processPayment(req.OrderNo, req.TradeNo, req.AmountCents, req.Currency, hex.EncodeToString(eventSum[:]), time.Now()); err != nil {
		a.orderError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS"})
}

func (a *App) mockPay(c *gin.Context) {
	var order Order
	if err := a.db.Where("order_no = ?", c.Param("order_no")).First(&order).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "订单不存在")
		return
	}
	tradeNo := randomID("MOCK")
	if err := a.processPayment(order.OrderNo, tradeNo, order.TotalAmountCents, order.Currency, keyedHash(a.config.PaymentMerchantKey, tradeNo), time.Now()); err != nil {
		a.orderError(c, err)
		return
	}
	_ = a.db.Where("id = ?", order.ID).First(&order).Error
	result, _ := a.orderResult(a.db, order, true)
	c.JSON(http.StatusOK, result)
}

func (a *App) adminListUsers(c *gin.Context) {
	var rows []User
	query := a.db.Order("id DESC")
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+q+"%", "%"+q+"%")
	}
	if err := paginate(c, query, &rows); err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (a *App) adminCreateUser(c *gin.Context) {
	var req struct{ Username, Email, Password, Role string }
	if !bindJSON(c, &req) {
		return
	}
	if len(req.Password) < 8 || strings.TrimSpace(req.Username) == "" || !strings.Contains(req.Email, "@") {
		fail(c, http.StatusBadRequest, "invalid_account", "账号信息不符合要求")
		return
	}
	if req.Role != "admin" {
		req.Role = "user"
	}
	hash, _ := passwordHash(req.Password)
	row := User{Username: strings.TrimSpace(req.Username), Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: hash, Role: req.Role, Status: "active"}
	if err := a.db.Create(&row).Error; err != nil {
		fail(c, http.StatusConflict, "account_exists", "用户已存在")
		return
	}
	a.audit(c, "user.create", "user", row.ID, "")
	c.JSON(http.StatusCreated, row)
}

func (a *App) adminUpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Status string `json:"status"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Status != "active" && req.Status != "disabled" {
		fail(c, http.StatusBadRequest, "invalid_status", "状态不正确")
		return
	}
	if err := a.db.Model(&User{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "更新失败")
		return
	}
	a.audit(c, "user.update", "user", uint(id), "")
	c.Status(http.StatusNoContent)
}

func (a *App) adminAdjustBalance(c *gin.Context) {
	userID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid_user", "用户不存在")
		return
	}
	var req struct {
		Direction      string `json:"direction"`
		AmountCents    int64  `json:"amount_cents"`
		Reason         string `json:"reason"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if (req.Direction != "in" && req.Direction != "out") || req.AmountCents <= 0 || strings.TrimSpace(req.Reason) == "" || len(req.IdempotencyKey) < 8 {
		fail(c, http.StatusBadRequest, "invalid_adjustment", "余额调整参数不正确")
		return
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	admin := currentUser(c)
	var ledger BalanceLedger
	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND idempotency_key = ?", userID64, req.IdempotencyKey).First(&ledger).Error; err == nil {
			return nil
		} else if !isNotFound(err) {
			return err
		}
		var user User
		if err := tx.First(&user, userID64).Error; err != nil {
			return err
		}
		change := req.AmountCents
		kind := "admin_credit"
		if req.Direction == "out" {
			change = -change
			kind = "admin_debit"
		}
		update := tx.Model(&User{}).Where("id = ? AND balance_cents + ? >= 0", user.ID, change).Updates(map[string]any{"balance_cents": gorm.Expr("balance_cents + ?", change), "version": gorm.Expr("version + 1")})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errInsufficientBalance
		}
		if err := tx.First(&user, user.ID).Error; err != nil {
			return err
		}
		ledger = BalanceLedger{UserID: user.ID, Type: kind, Direction: req.Direction, AmountCents: req.AmountCents, BalanceAfterCents: user.BalanceCents, Reason: req.Reason, OperatorID: &admin.ID, IdempotencyKey: req.IdempotencyKey}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		return tx.Create(&AuditLog{OperatorID: admin.ID, Action: "balance.adjust", TargetType: "user", TargetID: fmt.Sprint(user.ID), Reason: req.Reason, IP: c.ClientIP()}).Error
	})
	if err != nil {
		a.orderError(c, err)
		return
	}
	c.JSON(http.StatusOK, ledger)
}

func (a *App) audit(c *gin.Context, action, targetType string, targetID uint, reason string) {
	user := currentUser(c)
	_ = a.db.Create(&AuditLog{OperatorID: user.ID, Action: action, TargetType: targetType, TargetID: fmt.Sprint(targetID), Reason: reason, IP: c.ClientIP()}).Error
}
