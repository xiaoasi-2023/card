package main

import "testing"

func TestSeedDefaultCatalogIsIdempotentAndDoesNotCreateCards(t *testing.T) {
	app := testApp(t)
	qaPlatform := Platform{Code: "qa-seed-test", Name: "QA Seed Test", Status: "active"}
	if err := app.db.Create(&qaPlatform).Error; err != nil {
		t.Fatal(err)
	}
	qaProduct := Product{PlatformID: qaPlatform.ID, Name: "QA Product", Slug: "qa-seed-product", Status: "active"}
	if err := app.db.Create(&qaProduct).Error; err != nil {
		t.Fatal(err)
	}
	qaSKU := SKU{ProductID: qaProduct.ID, Name: "QA SKU", SalePriceCents: 100, Status: "active"}
	if err := app.db.Create(&qaSKU).Error; err != nil {
		t.Fatal(err)
	}
	qaBatch := CardBatch{SKUID: qaSKU.ID, TotalCount: 1, SuccessCount: 1, ImportedBy: 1}
	if err := app.db.Create(&qaBatch).Error; err != nil {
		t.Fatal(err)
	}
	qaCard := Card{
		SKUID: qaSKU.ID, BatchID: qaBatch.ID, SecretCiphertext: "test",
		SecretHash: "qa-seed-card", KeyVersion: 1, Status: "available",
	}
	if err := app.db.Create(&qaCard).Error; err != nil {
		t.Fatal(err)
	}

	for attempt, replace := range []bool{true, false} {
		if _, err := seedDefaultCatalog(app.db, replace); err != nil {
			t.Fatalf("seed attempt %d: %v", attempt+1, err)
		}
	}

	var activePlatformCount, activeProductCount, activeSKUCount, cardCount int64
	app.db.Model(&Platform{}).Where("status = ?", "active").Count(&activePlatformCount)
	app.db.Model(&Product{}).Where("status = ?", "active").Count(&activeProductCount)
	app.db.Model(&SKU{}).Where("status = ?", "active").Count(&activeSKUCount)
	app.db.Model(&Card{}).Count(&cardCount)
	if activePlatformCount != 7 || activeProductCount != 14 || activeSKUCount != 14 {
		t.Fatalf("active counts: platforms=%d products=%d skus=%d", activePlatformCount, activeProductCount, activeSKUCount)
	}
	if cardCount != 0 {
		t.Fatalf("seed created %d cards", cardCount)
	}

	var cheapCount, largeCount int64
	app.db.Model(&SKU{}).Where("status = ? AND sale_price_cents = ?", "active", 500).Count(&cheapCount)
	app.db.Model(&SKU{}).Where("status = ? AND sale_price_cents = ?", "active", 4500).Count(&largeCount)
	if cheapCount != 7 || largeCount != 7 {
		t.Fatalf("price counts: 500=%d 4500=%d", cheapCount, largeCount)
	}

	var qaPlatformCount, qaProductCount, qaSKUCount int64
	app.db.Model(&Platform{}).Where("code = ?", qaPlatform.Code).Count(&qaPlatformCount)
	app.db.Model(&Product{}).Where("slug = ?", qaProduct.Slug).Count(&qaProductCount)
	app.db.Model(&SKU{}).Where("id = ?", qaSKU.ID).Count(&qaSKUCount)
	if qaPlatformCount != 0 || qaProductCount != 0 || qaSKUCount != 0 {
		t.Fatalf("QA rows remain: platforms=%d products=%d skus=%d", qaPlatformCount, qaProductCount, qaSKUCount)
	}
}
