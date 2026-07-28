package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	cardStatusAvailable = "available"
	cardStatusSold      = "sold"
	cardStatusAllocated = "allocated"
	cardStatusReserved  = "reserved"
	cardStatusVoid      = "void"

	claimRateWindow = time.Minute
	claimRateLimit  = 20
	claimCodeRetry  = 8
)

type allocateCardsRequest struct {
	Count  int    `json:"count"`
	MarkAs string `json:"mark_as"`
}

// exportedCard 是管理端导出 / 取码响应项。
// 默认只给 claim_code（灌小铺）；include_secret=1 时才附带真卡密。
type exportedCard struct {
	ID            uint   `json:"id"`
	ClaimCode     string `json:"claim_code"`
	QueryPassword string `json:"query_password,omitempty"`
	Secret        string `json:"secret,omitempty"`
	Status        string `json:"status"`
}

func (a *App) adminExportSKUCards(c *gin.Context) {
	skuID := parseID(c)
	if skuID == 0 {
		return
	}
	var sku SKU
	if err := a.db.Preload("Product").First(&sku, skuID).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "SKU 不存在")
		return
	}
	limit := 1000
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			fail(c, http.StatusBadRequest, "invalid_limit", "limit 参数不正确")
			return
		}
		if value > 5000 {
			value = 5000
		}
		limit = value
	}
	includeSecret := queryTruthy(c.Query("include_secret"))

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	var cards []Card
	if err := a.db.Where("sku_id = ? AND status = ?", skuID, cardStatusAvailable).Order("id").Limit(limit).Find(&cards).Error; err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "查询失败")
		return
	}

	items := make([]exportedCard, 0, len(cards))
	claimCodes := make([]string, 0, len(cards))
	secrets := make([]string, 0, len(cards))
	for i := range cards {
		claimCode, queryPassword, err := a.ensureClaimCredentials(a.db, &cards[i], true)
		if err != nil {
			fail(c, http.StatusInternalServerError, "claim_code_failed", "生成领取码失败")
			return
		}
		item := exportedCard{
			ID:            cards[i].ID,
			ClaimCode:     claimCode,
			QueryPassword: queryPassword,
			Status:        cards[i].Status,
		}
		if includeSecret {
			secret, err := decryptString(a.config.CardEncryptKey, cards[i].SecretCiphertext)
			if err != nil {
				fail(c, http.StatusInternalServerError, "decrypt_failed", "卡密解密失败")
				return
			}
			item.Secret = secret
			secrets = append(secrets, secret)
		}
		items = append(items, item)
		claimCodes = append(claimCodes, claimCode)
	}

	a.audit(c, "card.export", "sku", skuID, fmt.Sprintf("count=%d include_secret=%v", len(items), includeSecret))
	payload := gin.H{
		"sku_id":       sku.ID,
		"sku_name":     sku.Name,
		"product_name": sku.Product.Name,
		"count":        len(items),
		"items":        items,
		"claim_codes":  claimCodes,
		// 兼容旧字段：默认返回领取码列表，不再把真卡密灌给小铺。
		"secrets": claimCodes,
	}
	if includeSecret {
		payload["real_secrets"] = secrets
	}
	c.JSON(http.StatusOK, payload)
}

func (a *App) adminAllocateSKUCards(c *gin.Context) {
	skuID := parseID(c)
	if skuID == 0 {
		return
	}
	var req allocateCardsRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Count < 1 {
		req.Count = 1
	}
	if req.Count > 100 {
		fail(c, http.StatusBadRequest, "invalid_count", "单次取码数量应为 1 至 100")
		return
	}
	status := strings.TrimSpace(req.MarkAs)
	if status == "" {
		status = cardStatusAllocated
	}
	if status != cardStatusSold && status != cardStatusAllocated {
		fail(c, http.StatusBadRequest, "invalid_mark_as", "mark_as 只能是 sold 或 allocated")
		return
	}
	includeSecret := queryTruthy(c.Query("include_secret"))

	var sku SKU
	if err := a.db.Preload("Product").First(&sku, skuID).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "SKU 不存在")
		return
	}

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	now := time.Now()
	var items []exportedCard
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var cards []Card
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("sku_id = ? AND status = ?", skuID, cardStatusAvailable).
			Order("id").Limit(req.Count).Find(&cards).Error; err != nil {
			return err
		}
		if len(cards) != req.Count {
			return errInsufficientStock
		}
		items = make([]exportedCard, 0, len(cards))
		for i := range cards {
			claimCode, queryPassword, err := a.ensureClaimCredentials(tx, &cards[i], true)
			if err != nil {
				return err
			}
			updates := map[string]any{
				"status":            status,
				"sold_at":           now,
				"reserved_order_id": nil,
				"reserved_until":    nil,
			}
			result := tx.Model(&Card{}).Where("id = ? AND status = ?", cards[i].ID, cardStatusAvailable).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientStock
			}
			item := exportedCard{
				ID:            cards[i].ID,
				ClaimCode:     claimCode,
				QueryPassword: queryPassword,
				Status:        status,
			}
			if includeSecret {
				secret, err := decryptString(a.config.CardEncryptKey, cards[i].SecretCiphertext)
				if err != nil {
					return err
				}
				item.Secret = secret
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errInsufficientStock) {
			fail(c, http.StatusConflict, "insufficient_stock", "可用库存不足")
			return
		}
		fail(c, http.StatusInternalServerError, "internal_error", "取码失败")
		return
	}

	claimCodes := make([]string, 0, len(items))
	ids := make([]uint, 0, len(items))
	secrets := make([]string, 0, len(items))
	for _, item := range items {
		claimCodes = append(claimCodes, item.ClaimCode)
		ids = append(ids, item.ID)
		if item.Secret != "" {
			secrets = append(secrets, item.Secret)
		}
	}
	afterJSON, _ := json.Marshal(map[string]any{"card_ids": ids, "status": status, "count": len(items)})
	admin := currentUser(c)
	_ = a.db.Create(&AuditLog{
		OperatorID: admin.ID,
		Action:     "card.allocate",
		TargetType: "sku",
		TargetID:   fmt.Sprint(skuID),
		AfterJSON:  string(afterJSON),
		Reason:     fmt.Sprintf("count=%d mark_as=%s", len(items), status),
		IP:         c.ClientIP(),
	}).Error

	payload := gin.H{
		"sku_id":       sku.ID,
		"sku_name":     sku.Name,
		"product_name": sku.Product.Name,
		"count":        len(items),
		"status":       status,
		"items":        items,
		"claim_codes":  claimCodes,
		// 兼容旧字段：默认是领取码，不是上游真卡密。
		"secrets":      claimCodes,
		"allocated_at": now.UTC().Format(time.RFC3339),
	}
	if includeSecret {
		payload["real_secrets"] = secrets
	}
	c.JSON(http.StatusOK, payload)
}

func (a *App) publicTrafficClaim(c *gin.Context) {
	if !a.allowClaim(c.ClientIP()) {
		c.Header("Retry-After", "60")
		fail(c, http.StatusTooManyRequests, "rate_limited", "请求过于频繁，请稍后再试")
		return
	}

	var req struct {
		ClaimCode     string `json:"claim_code"`
		QueryPassword string `json:"query_password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	claimCode := normalizeClaimCode(req.ClaimCode)
	queryPassword := strings.TrimSpace(req.QueryPassword)
	if claimCode == "" || len([]byte(claimCode)) > 128 {
		fail(c, http.StatusBadRequest, "invalid_claim_code", "领取码不正确")
		return
	}

	digest := keyedHash(a.config.CardHashKey, claimCode)
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	now := time.Now()
	var productName, skuName, secret string
	var claimedAt time.Time
	var replay bool
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var card Card
		// 终局路径：只按领取码哈希查找，不再把真卡密当 claim_code。
		if err := tx.Where("claim_code_hash = ?", digest).First(&card).Error; err != nil {
			if isNotFound(err) {
				return errInvalidOrder
			}
			return err
		}

		switch card.Status {
		case cardStatusAvailable, cardStatusAllocated:
			// available / allocated 都允许首次领取：
			// allocated 表示已从库存取出灌小铺，但用户尚未回站揭示真卡密。
			plain, err := decryptString(a.config.CardEncryptKey, card.SecretCiphertext)
			if err != nil {
				return err
			}
			updates := map[string]any{
				"status":            cardStatusSold,
				"sold_at":           now,
				"claimed_at":        now,
				"reserved_order_id": nil,
				"reserved_until":    nil,
			}
			if queryPassword != "" && strings.TrimSpace(card.QueryPasswordHash) == "" {
				hash, err := passwordHash(queryPassword)
				if err != nil {
					return err
				}
				updates["query_password_hash"] = hash
			}
			result := tx.Model(&Card{}).Where("id = ? AND status = ?", card.ID, card.Status).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errInsufficientStock
			}
			var sku SKU
			if err := tx.Preload("Product").First(&sku, card.SKUID).Error; err != nil {
				return err
			}
			productName = sku.Product.Name
			skuName = sku.Name
			secret = plain
			claimedAt = now
			detail, _ := json.Marshal(map[string]any{
				"card_id": card.ID, "sku_id": card.SKUID, "ip": c.ClientIP(), "mode": "claim_by_code",
			})
			return tx.Create(&AuditLog{
				OperatorID: 0,
				Action:     "card.public_claim",
				TargetType: "card",
				TargetID:   fmt.Sprint(card.ID),
				AfterJSON:  string(detail),
				Reason:     "public traffic claim",
				IP:         c.ClientIP(),
			}).Error

		case cardStatusSold:
			// 已领取：有查单密码且匹配时允许再次展示明文（售后）。
			if strings.TrimSpace(card.QueryPasswordHash) == "" || queryPassword == "" || !passwordMatches(card.QueryPasswordHash, queryPassword) {
				return errCredentialMismatch
			}
			plain, err := decryptString(a.config.CardEncryptKey, card.SecretCiphertext)
			if err != nil {
				return err
			}
			var sku SKU
			if err := tx.Preload("Product").First(&sku, card.SKUID).Error; err != nil {
				return err
			}
			productName = sku.Product.Name
			skuName = sku.Name
			secret = plain
			if card.ClaimedAt != nil {
				claimedAt = *card.ClaimedAt
			} else if card.SoldAt != nil {
				claimedAt = *card.SoldAt
			} else {
				claimedAt = now
			}
			replay = true
			detail, _ := json.Marshal(map[string]any{
				"card_id": card.ID, "sku_id": card.SKUID, "ip": c.ClientIP(), "mode": "claim_replay",
			})
			return tx.Create(&AuditLog{
				OperatorID: 0,
				Action:     "card.public_claim_replay",
				TargetType: "card",
				TargetID:   fmt.Sprint(card.ID),
				AfterJSON:  string(detail),
				Reason:     "public traffic claim replay",
				IP:         c.ClientIP(),
			}).Error

		case cardStatusReserved, cardStatusVoid:
			return errCredentialMismatch
		default:
			return errCredentialMismatch
		}
	})
	if err != nil {
		switch {
		case errors.Is(err, errInvalidOrder):
			fail(c, http.StatusNotFound, "claim_not_found", "领取码无效")
		case errors.Is(err, errCredentialMismatch):
			fail(c, http.StatusConflict, "claim_already_used", "领取码已使用")
		case errors.Is(err, errInsufficientStock):
			fail(c, http.StatusConflict, "claim_conflict", "领取冲突，请重试")
		default:
			fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"product_name": productName,
		"sku_name":     skuName,
		"secrets":      []string{secret},
		"claimed_at":   claimedAt.UTC().Format(time.RFC3339),
		"replay":       replay,
	})
}

func (a *App) allowClaim(ip string) bool {
	if ip == "" {
		ip = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-claimRateWindow)

	a.claimRateMu.Lock()
	defer a.claimRateMu.Unlock()
	if a.claimHits == nil {
		a.claimHits = map[string][]time.Time{}
	}
	hits := a.claimHits[ip]
	kept := hits[:0]
	for _, hit := range hits {
		if hit.After(cutoff) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= claimRateLimit {
		a.claimHits[ip] = kept
		return false
	}
	a.claimHits[ip] = append(kept, now)
	return true
}

// ensureClaimCredentials 保证卡片有领取码。
// withQueryPassword=true 时：若尚未设置查单密码则生成并返回明文一次；
// 已设置则不轮换（避免重复导出把小铺已发出的密码作废）。
func (a *App) ensureClaimCredentials(tx *gorm.DB, card *Card, withQueryPassword bool) (claimCode string, queryPassword string, err error) {
	if card == nil {
		return "", "", errors.New("card is nil")
	}
	claimCode = normalizeClaimCode(card.ClaimCode)
	if claimCode == "" || strings.TrimSpace(card.ClaimCodeHash) == "" {
		code, hash, genErr := a.allocateUniqueClaimCode(tx)
		if genErr != nil {
			return "", "", genErr
		}
		if err := tx.Model(&Card{}).Where("id = ?", card.ID).Updates(map[string]any{
			"claim_code":      code,
			"claim_code_hash": hash,
		}).Error; err != nil {
			return "", "", err
		}
		card.ClaimCode = code
		card.ClaimCodeHash = hash
		claimCode = code
	}

	if withQueryPassword && strings.TrimSpace(card.QueryPasswordHash) == "" {
		plain, hash, genErr := a.newQueryPasswordPair()
		if genErr != nil {
			return "", "", genErr
		}
		if err := tx.Model(&Card{}).Where("id = ?", card.ID).Update("query_password_hash", hash).Error; err != nil {
			return "", "", err
		}
		card.QueryPasswordHash = hash
		queryPassword = plain
	}
	return claimCode, queryPassword, nil
}

func (a *App) newQueryPasswordPair() (plain string, hash string, err error) {
	plain, err = generateQueryPassword()
	if err != nil {
		return "", "", err
	}
	hash, err = passwordHash(plain)
	if err != nil {
		return "", "", err
	}
	return plain, hash, nil
}

func queryTruthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}

// allocateUniqueClaimCode 生成尚未占用的领取码与哈希。
func (a *App) allocateUniqueClaimCode(tx *gorm.DB) (string, string, error) {
	for attempt := 0; attempt < claimCodeRetry; attempt++ {
		code, err := generateClaimCode()
		if err != nil {
			return "", "", err
		}
		hash := keyedHash(a.config.CardHashKey, code)
		var count int64
		if err := tx.Model(&Card{}).Where("claim_code_hash = ? OR claim_code = ?", hash, code).Count(&count).Error; err != nil {
			return "", "", err
		}
		if count == 0 {
			return code, hash, nil
		}
	}
	return "", "", errors.New("claim code generation exhausted")
}

// backfillClaimCodes 为历史卡密补齐领取码与查单密码哈希。
func (a *App) backfillClaimCodes() error {
	var cards []Card
	if err := a.db.Where("claim_code = '' OR claim_code IS NULL OR claim_code_hash = '' OR claim_code_hash IS NULL").Find(&cards).Error; err != nil {
		return err
	}
	for i := range cards {
		// 仅补领取码；查单密码留给导出/取码或用户首次领取时绑定
		if _, _, err := a.ensureClaimCredentials(a.db, &cards[i], false); err != nil {
			return err
		}
	}
	return nil
}
