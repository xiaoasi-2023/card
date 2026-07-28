package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *App) adminListPlatforms(c *gin.Context) {
	var rows []Platform
	if err := paginate(c, a.db.Order("sort DESC, id"), &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (a *App) adminCreatePlatform(c *gin.Context) {
	var req Platform
	if !bindJSON(c, &req) {
		return
	}
	req.ID = 0
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		fail(c, 400, "invalid_platform", "平台参数不正确")
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if err := a.db.Create(&req).Error; err != nil {
		fail(c, 409, "platform_exists", "平台编码已存在")
		return
	}
	a.audit(c, "platform.create", "platform", req.ID, "")
	c.JSON(http.StatusCreated, req)
}

func (a *App) adminUpdatePlatform(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var req Platform
	if !bindJSON(c, &req) {
		return
	}
	updates := map[string]any{"code": strings.TrimSpace(req.Code), "name": strings.TrimSpace(req.Name), "website": req.Website, "status": req.Status, "sort": req.Sort}
	if updates["code"] == "" || updates["name"] == "" || (req.Status != "active" && req.Status != "disabled") {
		fail(c, 400, "invalid_platform", "平台参数不正确")
		return
	}
	if err := a.db.Model(&Platform{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		fail(c, 409, "update_failed", "平台更新失败")
		return
	}
	a.audit(c, "platform.update", "platform", id, "")
	c.Status(http.StatusNoContent)
}

func (a *App) adminDeletePlatform(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var count int64
	_ = a.db.Model(&Product{}).Where("platform_id = ?", id).Count(&count).Error
	if count > 0 {
		fail(c, 409, "platform_in_use", "平台下已有商品，不能删除")
		return
	}
	if a.db.Delete(&Platform{}, id).RowsAffected == 0 {
		fail(c, 404, "not_found", "平台不存在")
		return
	}
	a.audit(c, "platform.delete", "platform", id, "")
	c.Status(http.StatusNoContent)
}

func (a *App) adminListProducts(c *gin.Context) {
	var rows []Product
	query := a.db.Preload("Platform").Order("sort DESC, id DESC")
	if v := c.Query("platform_id"); v != "" {
		query = query.Where("platform_id = ?", v)
	}
	if err := paginate(c, query, &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (a *App) adminCreateProduct(c *gin.Context) {
	var req Product
	if !bindJSON(c, &req) {
		return
	}
	req.ID = 0
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.PlatformID == 0 || req.Name == "" || req.Slug == "" {
		fail(c, 400, "invalid_product", "商品参数不正确")
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if err := a.db.Create(&req).Error; err != nil {
		fail(c, 409, "product_exists", "商品标识已存在或平台无效")
		return
	}
	a.audit(c, "product.create", "product", req.ID, "")
	c.JSON(201, req)
}

func (a *App) adminUpdateProduct(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var req Product
	if !bindJSON(c, &req) {
		return
	}
	if req.PlatformID == 0 || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Slug) == "" || (req.Status != "active" && req.Status != "disabled") {
		fail(c, 400, "invalid_product", "商品参数不正确")
		return
	}
	updates := map[string]any{"platform_id": req.PlatformID, "name": strings.TrimSpace(req.Name), "slug": strings.TrimSpace(req.Slug), "description": req.Description, "cover_url": req.CoverURL, "status": req.Status, "sort": req.Sort}
	if err := a.db.Model(&Product{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		fail(c, 409, "update_failed", "商品更新失败")
		return
	}
	a.audit(c, "product.update", "product", id, "")
	c.Status(204)
}

func (a *App) adminDeleteProduct(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var count int64
	_ = a.db.Model(&SKU{}).Where("product_id = ?", id).Count(&count).Error
	if count > 0 {
		fail(c, 409, "product_in_use", "商品下已有 SKU，不能删除")
		return
	}
	if a.db.Delete(&Product{}, id).RowsAffected == 0 {
		fail(c, 404, "not_found", "商品不存在")
		return
	}
	a.audit(c, "product.delete", "product", id, "")
	c.Status(204)
}

func (a *App) adminListSKUs(c *gin.Context) {
	var rows []SKU
	query := a.db.Preload("Product").Order("id DESC")
	if v := c.Query("product_id"); v != "" {
		query = query.Where("product_id = ?", v)
	}
	if err := paginate(c, query, &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (a *App) adminCreateSKU(c *gin.Context) {
	var req SKU
	if !bindJSON(c, &req) {
		return
	}
	req.ID = 0
	req.Name = strings.TrimSpace(req.Name)
	if req.ProductID == 0 || req.Name == "" || req.SalePriceCents <= 0 {
		fail(c, 400, "invalid_sku", "SKU 参数不正确")
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if err := a.db.Create(&req).Error; err != nil {
		fail(c, 409, "create_failed", "SKU 创建失败")
		return
	}
	a.audit(c, "sku.create", "sku", req.ID, "")
	c.JSON(201, req)
}

func (a *App) adminUpdateSKU(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var req SKU
	if !bindJSON(c, &req) {
		return
	}
	if req.ProductID == 0 || strings.TrimSpace(req.Name) == "" || req.SalePriceCents <= 0 || (req.Status != "active" && req.Status != "disabled") {
		fail(c, 400, "invalid_sku", "SKU 参数不正确")
		return
	}
	updates := map[string]any{"product_id": req.ProductID, "name": strings.TrimSpace(req.Name), "attrs_json": req.AttrsJSON, "sale_price_cents": req.SalePriceCents, "status": req.Status}
	if err := a.db.Model(&SKU{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		fail(c, 409, "update_failed", "SKU 更新失败")
		return
	}
	a.audit(c, "sku.update", "sku", id, "")
	c.Status(204)
}

func (a *App) adminDeleteSKU(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var count int64
	_ = a.db.Model(&Card{}).Where("sku_id = ?", id).Count(&count).Error
	if count > 0 {
		fail(c, 409, "sku_in_use", "SKU 已有库存，不能删除")
		return
	}
	if a.db.Delete(&SKU{}, id).RowsAffected == 0 {
		fail(c, 404, "not_found", "SKU 不存在")
		return
	}
	a.audit(c, "sku.delete", "sku", id, "")
	c.Status(204)
}

type importCardsRequest struct {
	SKUID    uint     `json:"sku_id"`
	Filename string   `json:"filename"`
	Cards    []string `json:"cards"`
	Content  string   `json:"content"`
}

func (a *App) adminImportCards(c *gin.Context) {
	var req importCardsRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.SKUID == 0 {
		fail(c, 400, "invalid_batch", "请选择 SKU")
		return
	}
	lines := req.Cards
	if len(lines) == 0 && req.Content != "" {
		lines = strings.Split(strings.ReplaceAll(req.Content, "\r\n", "\n"), "\n")
	}
	if len(lines) == 0 || len(lines) > 10000 {
		fail(c, 400, "invalid_batch", "单批卡密数量应为 1 至 10000")
		return
	}
	admin := currentUser(c)
	batch := CardBatch{SKUID: req.SKUID, Filename: req.Filename, TotalCount: len(lines), ImportedBy: admin.ID}
	invalidLines := []int{}
	duplicateLines := []int{}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var sku SKU
		if err := tx.First(&sku, req.SKUID).Error; err != nil {
			return err
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		seen := map[string]struct{}{}
		for index, raw := range lines {
			secret := strings.TrimSpace(raw)
			if secret == "" || len([]byte(secret)) > 4096 {
				batch.InvalidCount++
				invalidLines = append(invalidLines, index+1)
				continue
			}
			digest := keyedHash(a.config.CardHashKey, secret)
			if _, ok := seen[digest]; ok {
				batch.DuplicateCount++
				duplicateLines = append(duplicateLines, index+1)
				continue
			}
			seen[digest] = struct{}{}
			var count int64
			if err := tx.Model(&Card{}).Where("secret_hash = ?", digest).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				batch.DuplicateCount++
				duplicateLines = append(duplicateLines, index+1)
				continue
			}
			ciphertext, err := encryptString(a.config.CardEncryptKey, secret)
			if err != nil {
				return err
			}
			claimCode, claimHash, err := a.allocateUniqueClaimCode(tx)
			if err != nil {
				return err
			}
			row := Card{
				SKUID:            req.SKUID,
				BatchID:          batch.ID,
				SecretCiphertext: ciphertext,
				SecretHash:       digest,
				ClaimCode:        claimCode,
				ClaimCodeHash:    claimHash,
				KeyVersion:       1,
				Status:           "available",
			}
if err := tx.Create(&row).Error; err != nil {
				return err
			}
			batch.SuccessCount++
		}
		return tx.Model(&batch).Updates(map[string]any{"success_count": batch.SuccessCount, "duplicate_count": batch.DuplicateCount, "invalid_count": batch.InvalidCount}).Error
	})
	if err != nil {
		fail(c, 400, "import_failed", "卡密导入失败")
		return
	}
	a.audit(c, "card.import", "card_batch", batch.ID, "")
	c.JSON(201, gin.H{"batch": batch, "invalid_lines": invalidLines, "duplicate_lines": duplicateLines})
}

func (a *App) adminGetBatch(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var row CardBatch
	if err := a.db.First(&row, id).Error; err != nil {
		fail(c, 404, "not_found", "导入批次不存在")
		return
	}
	c.JSON(200, row)
}

type adminCardView struct {
	ID              uint   `json:"id"`
	SKUID           uint   `json:"sku_id"`
	BatchID         uint   `json:"batch_id"`
	Status          string `json:"status"`
	ClaimCode       string `json:"claim_code,omitempty"`
	Secret          string `json:"secret"`
	ReservedOrderID *uint  `json:"reserved_order_id,omitempty"`
	SoldOrderID     *uint  `json:"sold_order_id,omitempty"`
	ClaimedAt       string `json:"claimed_at,omitempty"`
}

func (a *App) adminListCards(c *gin.Context) {
	var rows []Card
	query := a.db.Order("id DESC")
	if v := c.Query("sku_id"); v != "" {
		query = query.Where("sku_id = ?", v)
	}
	if v := c.Query("status"); v != "" {
		query = query.Where("status = ?", v)
	}
	if err := paginate(c, query, &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	views := make([]adminCardView, 0, len(rows))
	for _, row := range rows {
		secret, err := decryptString(a.config.CardEncryptKey, row.SecretCiphertext)
		if err != nil {
			fail(c, 500, "decrypt_failed", "卡密解密失败")
			return
		}
		view := adminCardView{
			ID:              row.ID,
			SKUID:           row.SKUID,
			BatchID:         row.BatchID,
			Status:          row.Status,
			ClaimCode:       row.ClaimCode,
			Secret:          secret,
			ReservedOrderID: row.ReservedOrderID,
			SoldOrderID:     row.SoldOrderID,
		}
		if row.ClaimedAt != nil {
			view.ClaimedAt = row.ClaimedAt.UTC().Format(time.RFC3339)
		}
		views = append(views, view)
	}
	a.audit(c, "card.list", "card", 0, "")
	c.JSON(200, gin.H{"items": views})
}

func (a *App) adminUpdateCard(c *gin.Context) {
	id := parseID(c)
	if id == 0 {
		return
	}
	var req struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Status != "void" || strings.TrimSpace(req.Reason) == "" {
		fail(c, 400, "invalid_card_update", "只能作废未售卡密且必须填写原因")
		return
	}
	result := a.db.Model(&Card{}).Where("id = ? AND status = ?", id, "available").Update("status", "void")
	if result.Error != nil || result.RowsAffected != 1 {
		fail(c, 409, "card_not_available", "卡密不是可作废状态")
		return
	}
	a.audit(c, "card.void", "card", id, req.Reason)
	c.Status(204)
}

func (a *App) adminListOrders(c *gin.Context) {
	var rows []Order
	query := a.db.Order("id DESC")
	if v := c.Query("status"); v != "" {
		query = query.Where("status = ?", v)
	}
	if v := c.Query("payment_method"); v != "" {
		query = query.Where("payment_method = ?", v)
	}
	if v := strings.TrimSpace(c.Query("q")); v != "" {
		like := "%" + v + "%"
		query = query.Where(`order_no LIKE ?
			OR id IN (SELECT order_id FROM order_items WHERE product_name_snapshot LIKE ? OR sku_name_snapshot LIKE ?)
			OR user_id IN (SELECT id FROM users WHERE username LIKE ? OR email LIKE ?)`, like, like, like, like, like)
	}
	if err := paginate(c, query, &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		result, err := a.orderResult(a.db, row, false)
		if err != nil {
			fail(c, 500, "internal_error", "查询失败")
			return
		}
		item := gin.H{"order": result.Order, "item": result.Item}
		if row.UserID != nil {
			var user User
			if err := a.db.First(&user, *row.UserID).Error; err != nil {
				fail(c, 500, "internal_error", "查询失败")
				return
			}
			item["user"] = user
		} else if row.ContactCiphertext != "" {
			contact, err := decryptString(a.config.ContactEncryptKey, row.ContactCiphertext)
			if err != nil {
				fail(c, 500, "decrypt_failed", "联系方式解密失败")
				return
			}
			item["contact_masked"] = maskContact(contact)
		}
		items = append(items, item)
	}
	c.JSON(200, gin.H{"items": items})
}

func maskContact(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}

func (a *App) adminGetOrder(c *gin.Context) {
	var row Order
	if err := a.db.Where("order_no = ?", c.Param("order_no")).First(&row).Error; err != nil {
		fail(c, 404, "not_found", "订单不存在")
		return
	}
	result, err := a.orderResult(a.db, row, true)
	if err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	if row.UserID == nil && row.ContactCiphertext != "" {
		result.Contact, err = decryptString(a.config.ContactEncryptKey, row.ContactCiphertext)
		if err != nil {
			fail(c, 500, "decrypt_failed", "联系方式解密失败")
			return
		}
	}
	a.audit(c, "order.view", "order", row.ID, "")
	c.JSON(200, result)
}

func (a *App) adminListPayments(c *gin.Context) {
	var rows []Payment
	query := a.db.Order("id DESC")
	if v := c.Query("status"); v != "" {
		query = query.Where("status = ?", v)
	}
	if err := paginate(c, query, &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (a *App) adminListAuditLogs(c *gin.Context) {
	var rows []AuditLog
	if err := paginate(c, a.db.Order("id DESC"), &rows); err != nil {
		fail(c, 500, "internal_error", "查询失败")
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func parseID(c *gin.Context) uint {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		fail(c, 400, "invalid_id", "编号不正确")
		return 0
	}
	return uint(value)
}
