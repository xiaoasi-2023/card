package main

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type catalogPlatformSpec struct {
	Code    string
	Name    string
	Website string
	Sort    int
}

type catalogProductSpec struct {
	Suffix         string
	Name           string
	Description    string
	SalePriceCents int64
	AttrsJSON      string
	Sort           int
}

type CatalogSeedResult struct {
	PlatformsUpserted int
	ProductsUpserted  int
	SKUsUpserted      int
	PlatformsPurged   int64
	ProductsPurged    int64
	SKUsPurged        int64
	CardBatchesPurged int64
	CardsPurged       int64
	OrdersPurged      int64
}

var defaultCatalogPlatforms = []catalogPlatformSpec{
	{Code: "cliproxy", Name: "CliProxy", Website: "https://cliproxy.com/", Sort: 70},
	{Code: "kookeey", Name: "Kookeey", Website: "https://www.kookeey.com/", Sort: 60},
	{Code: "b2proxy", Name: "B2Proxy", Website: "http://b2proxy.com/", Sort: 50},
	{Code: "711proxy", Name: "711Proxy", Website: "https://711proxy.com/", Sort: 40},
	{Code: "bunnyproxy", Name: "BunnyProxy", Website: "https://app.bunnyproxy.com/", Sort: 20},
	{Code: "udealproxy", Name: "UdealProxy", Website: "http://www.udealproxy.com/", Sort: 10},
}

var defaultCatalogProducts = []catalogProductSpec{
	{
		Suffix:         "1g",
		Name:           "1G 流量",
		Description:    "1G 代理流量 CDK，购买后在订单中直接查看卡密。",
		SalePriceCents: 500,
		AttrsJSON:      `{"traffic_gb":1}`,
		Sort:           0,
	},
	{
		Suffix:         "10g",
		Name:           "10G 流量",
		Description:    "10G 代理流量 CDK，购买后在订单中直接查看卡密。",
		SalePriceCents: 4500,
		AttrsJSON:      `{"traffic_gb":10}`,
		Sort:           0,
	},
}

func seedDefaultCatalog(db *gorm.DB, replace bool) (CatalogSeedResult, error) {
	result := CatalogSeedResult{}
	err := db.Transaction(func(tx *gorm.DB) error {
		if replace {
			purged, err := purgeCatalogData(tx)
			if err != nil {
				return err
			}
			result.PlatformsPurged = purged.PlatformsPurged
			result.ProductsPurged = purged.ProductsPurged
			result.SKUsPurged = purged.SKUsPurged
			result.CardBatchesPurged = purged.CardBatchesPurged
			result.CardsPurged = purged.CardsPurged
			result.OrdersPurged = purged.OrdersPurged
		}

		for _, platformSpec := range defaultCatalogPlatforms {
			platform, err := upsertCatalogPlatform(tx, platformSpec)
			if err != nil {
				return err
			}
			result.PlatformsUpserted++

			for _, productSpec := range defaultCatalogProducts {
				product, err := upsertCatalogProduct(tx, platform, productSpec)
				if err != nil {
					return err
				}
				result.ProductsUpserted++
				if err := upsertCatalogSKU(tx, product, productSpec); err != nil {
					return err
				}
				result.SKUsUpserted++
			}
		}
		return nil
	})
	if err != nil {
		return CatalogSeedResult{}, err
	}
	return result, nil
}

func purgeCatalogData(tx *gorm.DB) (CatalogSeedResult, error) {
	result := CatalogSeedResult{}
	var platformIDs, productIDs, skuIDs, orderIDs []uint
	if err := tx.Model(&Platform{}).Pluck("id", &platformIDs).Error; err != nil {
		return result, fmt.Errorf("list platforms for purge: %w", err)
	}
	if len(platformIDs) == 0 {
		return result, nil
	}
	if err := tx.Model(&Product{}).Where("platform_id IN ?", platformIDs).Pluck("id", &productIDs).Error; err != nil {
		return result, fmt.Errorf("list products for purge: %w", err)
	}
	if len(productIDs) > 0 {
		if err := tx.Model(&SKU{}).Where("product_id IN ?", productIDs).Pluck("id", &skuIDs).Error; err != nil {
			return result, fmt.Errorf("list SKUs for purge: %w", err)
		}
	}
	if len(skuIDs) > 0 {
		if err := tx.Model(&OrderItem{}).Distinct("order_id").Where("sku_id IN ?", skuIDs).Pluck("order_id", &orderIDs).Error; err != nil {
			return result, fmt.Errorf("list orders for purge: %w", err)
		}
		if len(orderIDs) > 0 {
			if err := tx.Where("order_id IN ?", orderIDs).Delete(&OrderCard{}).Error; err != nil {
				return result, fmt.Errorf("purge order cards: %w", err)
			}
			if err := tx.Where("order_id IN ?", orderIDs).Delete(&Payment{}).Error; err != nil {
				return result, fmt.Errorf("purge payments: %w", err)
			}
			if err := tx.Where("order_id IN ?", orderIDs).Delete(&OrderEvent{}).Error; err != nil {
				return result, fmt.Errorf("purge order events: %w", err)
			}
			if err := tx.Where("ref_type = ? AND ref_id IN ?", "order", orderIDs).Delete(&BalanceLedger{}).Error; err != nil {
				return result, fmt.Errorf("purge order ledgers: %w", err)
			}
			if err := tx.Where("order_id IN ?", orderIDs).Delete(&OrderItem{}).Error; err != nil {
				return result, fmt.Errorf("purge order items: %w", err)
			}
			orders := tx.Where("id IN ?", orderIDs).Delete(&Order{})
			if orders.Error != nil {
				return result, fmt.Errorf("purge orders: %w", orders.Error)
			}
			result.OrdersPurged = orders.RowsAffected
		}
		cards := tx.Where("sku_id IN ?", skuIDs).Delete(&Card{})
		if cards.Error != nil {
			return result, fmt.Errorf("purge cards: %w", cards.Error)
		}
		result.CardsPurged = cards.RowsAffected
		batches := tx.Where("sku_id IN ?", skuIDs).Delete(&CardBatch{})
		if batches.Error != nil {
			return result, fmt.Errorf("purge card batches: %w", batches.Error)
		}
		result.CardBatchesPurged = batches.RowsAffected
		skus := tx.Where("id IN ?", skuIDs).Delete(&SKU{})
		if skus.Error != nil {
			return result, fmt.Errorf("purge SKUs: %w", skus.Error)
		}
		result.SKUsPurged = skus.RowsAffected
	}
	if len(productIDs) > 0 {
		products := tx.Where("id IN ?", productIDs).Delete(&Product{})
		if products.Error != nil {
			return result, fmt.Errorf("purge products: %w", products.Error)
		}
		result.ProductsPurged = products.RowsAffected
	}
	platforms := tx.Where("id IN ?", platformIDs).Delete(&Platform{})
	if platforms.Error != nil {
		return result, fmt.Errorf("purge platforms: %w", platforms.Error)
	}
	result.PlatformsPurged = platforms.RowsAffected
	audits := tx.Where("target_type IN ?", []string{"platform", "product", "sku", "card", "card_batch", "order"}).Delete(&AuditLog{})
	if audits.Error != nil {
		return result, fmt.Errorf("purge catalog audit logs: %w", audits.Error)
	}
	return result, nil
}

func upsertCatalogPlatform(tx *gorm.DB, spec catalogPlatformSpec) (Platform, error) {
	var platform Platform
	err := tx.Where("code = ?", spec.Code).First(&platform).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		platform = Platform{
			Code: spec.Code, Name: spec.Name, Website: spec.Website,
			Status: "active", Sort: spec.Sort,
		}
		if err := tx.Create(&platform).Error; err != nil {
			return Platform{}, fmt.Errorf("create platform %s: %w", spec.Code, err)
		}
		return platform, nil
	}
	if err != nil {
		return Platform{}, fmt.Errorf("find platform %s: %w", spec.Code, err)
	}
	if err := tx.Model(&platform).Updates(map[string]any{
		"name": spec.Name, "website": spec.Website, "status": "active", "sort": spec.Sort,
	}).Error; err != nil {
		return Platform{}, fmt.Errorf("update platform %s: %w", spec.Code, err)
	}
	return platform, nil
}

func upsertCatalogProduct(tx *gorm.DB, platform Platform, spec catalogProductSpec) (Product, error) {
	slug := platform.Code + "-" + spec.Suffix
	var product Product
	err := tx.Where("slug = ?", slug).First(&product).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		product = Product{
			PlatformID:  platform.ID,
			Name:        spec.Name,
			Slug:        slug,
			Description: platform.Name + " " + spec.Description,
			Status:      "active",
			Sort:        spec.Sort,
		}
		if err := tx.Create(&product).Error; err != nil {
			return Product{}, fmt.Errorf("create product %s: %w", slug, err)
		}
		return product, nil
	}
	if err != nil {
		return Product{}, fmt.Errorf("find product %s: %w", slug, err)
	}
	if err := tx.Model(&product).Updates(map[string]any{
		"platform_id": platform.ID,
		"name":        spec.Name,
		"description": platform.Name + " " + spec.Description,
		"status":      "active",
		"sort":        spec.Sort,
	}).Error; err != nil {
		return Product{}, fmt.Errorf("update product %s: %w", slug, err)
	}
	return product, nil
}

func upsertCatalogSKU(tx *gorm.DB, product Product, spec catalogProductSpec) error {
	var sku SKU
	err := tx.Where("product_id = ? AND name = ?", product.ID, spec.Name).First(&sku).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sku = SKU{
			ProductID: product.ID,
			Name:      spec.Name, AttrsJSON: spec.AttrsJSON,
			SalePriceCents: spec.SalePriceCents, Status: "active",
		}
		if err := tx.Create(&sku).Error; err != nil {
			return fmt.Errorf("create SKU for %s: %w", product.Slug, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("find SKU for %s: %w", product.Slug, err)
	}
	if err := tx.Model(&sku).Updates(map[string]any{
		"attrs_json": spec.AttrsJSON, "sale_price_cents": spec.SalePriceCents, "status": "active",
	}).Error; err != nil {
		return fmt.Errorf("update SKU for %s: %w", product.Slug, err)
	}
	return nil
}
