package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	config := Config{
		Env: "test", DatabasePath: filepath.Join(t.TempDir(), "card.db"), BaseURL: "http://example.test", JWTSecret: "jwt-test-key",
		CardEncryptKey: "card-encrypt-test", CardHashKey: "card-hash-test", ContactEncryptKey: "contact-encrypt-test", ContactHashKey: "contact-hash-test",
		PaymentProvider: "mock", PaymentMerchantID: "mock-merchant", PaymentMerchantKey: "payment-test-key", PaymentTimeout: time.Minute, MaxPurchaseQuantity: 100,
		RegistrationEnabled: true, SMTPHost: "smtp.example.test", SMTPPort: 465, SMTPUsername: "sender@example.test", SMTPPassword: "test-only-password",
		SMTPFrom: "sender@example.test", SMTPTLS: true, RegistrationCodeHashKey: "registration-code-test-key", RegistrationCodeTTL: 10 * time.Minute,
		RegistrationCodeResendInterval: time.Minute, RegistrationCodeMaxAttempts: 5,
	}
	app, err := newApp(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := app.db.DB()
		_ = sqlDB.Close()
	})
	return app
}

type fakeRegistrationMailer struct {
	code      string
	recipient string
	calls     int
	err       error
}

func (m *fakeRegistrationMailer) SendRegistrationCode(_ context.Context, recipient, code string, _ time.Duration) error {
	m.calls++
	m.recipient = recipient
	m.code = code
	return m.err
}

type testFixture struct {
	User User
	SKU  SKU
}

func seedFixture(t *testing.T, app *App, balance int64, secrets ...string) testFixture {
	t.Helper()
	hash, _ := passwordHash("password123")
	user := User{Username: randomID("user"), Email: randomID("mail") + "@example.test", PasswordHash: hash, Role: "user", Status: "active", BalanceCents: balance}
	if err := app.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	platform := Platform{Code: randomID("platform"), Name: "测试平台", Status: "active"}
	if err := app.db.Create(&platform).Error; err != nil {
		t.Fatal(err)
	}
	product := Product{PlatformID: platform.ID, Name: "测试商品", Slug: randomID("product"), Status: "active"}
	if err := app.db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	sku := SKU{ProductID: product.ID, Name: "标准规格", SalePriceCents: 700, Status: "active"}
	if err := app.db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	batch := CardBatch{SKUID: sku.ID, TotalCount: len(secrets), SuccessCount: len(secrets), ImportedBy: user.ID}
	if err := app.db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	for i, secret := range secrets {
		ciphertext, _ := encryptString(app.config.CardEncryptKey, secret)
		claimCode := normalizeClaimCode("TRAF-TEST-" + randomID("C") + "-" + strconv.Itoa(i+1))
		card := Card{
			SKUID:            sku.ID,
			BatchID:          batch.ID,
			SecretCiphertext: ciphertext,
			SecretHash:       keyedHash(app.config.CardHashKey, secret),
			ClaimCode:        claimCode,
			ClaimCodeHash:    keyedHash(app.config.CardHashKey, claimCode),
			KeyVersion:       1,
			Status:           "available",
		}
		if err := app.db.Create(&card).Error; err != nil {
			t.Fatal(err)
		}
	}
	return testFixture{User: user, SKU: sku}
}

func firstClaimCode(t *testing.T, app *App, skuID uint) (string, Card) {
	t.Helper()
	var card Card
	if err := app.db.Where("sku_id = ?", skuID).Order("id").First(&card).Error; err != nil {
		t.Fatal(err)
	}
	if card.ClaimCode == "" {
		t.Fatal("missing claim code")
	}
	return card.ClaimCode, card
}

func TestBalancePurchaseIsAtomicAndIdempotent(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 2000, "CARD-A", "CARD-B")
	req := createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 2, PaymentMethod: "balance", IdempotencyKey: "balance-idem-001"}
	result, created, err := app.createOrder(&fixture.User, req)
	if err != nil || !created {
		t.Fatalf("first purchase: created=%v err=%v", created, err)
	}
	if result.Order.Status != "completed" || len(result.Cards) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	result, created, err = app.createOrder(&fixture.User, req)
	if err != nil || created || len(result.Cards) != 2 {
		t.Fatalf("idempotent retry: created=%v err=%v", created, err)
	}
	var user User
	app.db.First(&user, fixture.User.ID)
	if user.BalanceCents != 600 {
		t.Fatalf("balance=%d, want 600", user.BalanceCents)
	}
	var ledgerCount, orderCount, soldCount int64
	app.db.Model(&BalanceLedger{}).Where("user_id = ?", user.ID).Count(&ledgerCount)
	app.db.Model(&Order{}).Where("user_id = ?", user.ID).Count(&orderCount)
	app.db.Model(&Card{}).Where("status = ?", "sold").Count(&soldCount)
	if ledgerCount != 1 || orderCount != 1 || soldCount != 2 {
		t.Fatalf("ledger=%d order=%d sold=%d", ledgerCount, orderCount, soldCount)
	}
	_, _, err = app.createOrder(&fixture.User, createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 1, PaymentMethod: "balance", IdempotencyKey: "balance-idem-002"})
	if err != errInsufficientStock {
		t.Fatalf("want stock error, got %v", err)
	}
	app.db.First(&user, fixture.User.ID)
	if user.BalanceCents != 600 {
		t.Fatalf("failed order changed balance to %d", user.BalanceCents)
	}
}

func TestGuestQueryRequiresAllCredentialsAndHidesPendingCards(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "GUEST-CARD")
	req := createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 1, PaymentMethod: "online", IdempotencyKey: "guest-idem-001", ContactType: "qq", Contact: "12345678", QueryPassword: "query789"}
	created, _, err := app.createOrder(nil, req)
	if err != nil {
		t.Fatal(err)
	}

	wrong := map[string]any{"order_no": created.Order.OrderNo, "contact_type": "qq", "contact": "12345678", "query_password": "wrongpass"}
	res := postJSON(app, "/api/v1/guest/orders/query", wrong)
	if res.Code != http.StatusForbidden {
		t.Fatalf("wrong password status=%d body=%s", res.Code, res.Body.String())
	}
	correct := map[string]any{"order_no": created.Order.OrderNo, "contact_type": "qq", "contact": "12345678", "query_password": "query789"}
	res = postJSON(app, "/api/v1/guest/orders/query", correct)
	if res.Code != http.StatusOK || bytes.Contains(res.Body.Bytes(), []byte("GUEST-CARD")) {
		t.Fatalf("pending response=%d %s", res.Code, res.Body.String())
	}

	if err := app.processPayment(created.Order.OrderNo, "trade-guest-1", created.Order.TotalAmountCents, "CNY", "event-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	res = postJSON(app, "/api/v1/guest/orders/query", correct)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte("GUEST-CARD")) {
		t.Fatalf("completed response=%d %s", res.Code, res.Body.String())
	}
}

func TestPaymentCallbackIsIdempotentAndInventoryUnique(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "ONLINE-A", "ONLINE-B")
	result, _, err := app.createOrder(&fixture.User, createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 2, PaymentMethod: "online", IdempotencyKey: "online-idem-001"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := app.processPayment(result.Order.OrderNo, "trade-online-1", result.Order.TotalAmountCents, "CNY", "event-online-1", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	var eventCount, soldCount, orderCardCount int64
	app.db.Model(&OrderEvent{}).Where("order_id = ? AND event_type = ?", result.Order.ID, "payment_success").Count(&eventCount)
	app.db.Model(&Card{}).Where("sold_order_id = ? AND status = ?", result.Order.ID, "sold").Count(&soldCount)
	app.db.Model(&OrderCard{}).Where("order_id = ?", result.Order.ID).Count(&orderCardCount)
	if eventCount != 1 || soldCount != 2 || orderCardCount != 2 {
		t.Fatalf("events=%d sold=%d links=%d", eventCount, soldCount, orderCardCount)
	}
}

func TestPaymentWebhookRequiresValidSignature(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "SIGNED-CARD")
	result, _, err := app.createOrder(&fixture.User, createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 1, PaymentMethod: "online", IdempotencyKey: "signed-idem-001"})
	if err != nil {
		t.Fatal(err)
	}
	callback := paymentCallbackRequest{MerchantID: app.config.PaymentMerchantID, OrderNo: result.Order.OrderNo, TradeNo: "trade-signed-1", AmountCents: result.Order.TotalAmountCents, Currency: "CNY", Timestamp: time.Now().Unix()}
	response := postSignedCallback(app, callback, "bad-signature")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid signature status=%d body=%s", response.Code, response.Body.String())
	}
	signature := signPayment(app.config.PaymentMerchantKey, callback)
	for i := 0; i < 2; i++ {
		response = postSignedCallback(app, callback, signature)
		if response.Code != http.StatusOK {
			t.Fatalf("callback %d status=%d body=%s", i, response.Code, response.Body.String())
		}
	}
	var order Order
	app.db.First(&order, result.Order.ID)
	if order.Status != "completed" {
		t.Fatalf("order status=%s", order.Status)
	}
}

func TestExpiredReservationReturnsInventory(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "EXPIRE-CARD")
	result, _, err := app.createOrder(nil, createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 1, PaymentMethod: "online", IdempotencyKey: "expire-idem-001", ContactType: "phone", Contact: "+86 138-0013-8000", QueryPassword: "query789"})
	if err != nil {
		t.Fatal(err)
	}
	count, err := app.releaseExpired(result.Order.ExpiresAt.Add(time.Second))
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	var card Card
	app.db.Where("sku_id = ?", fixture.SKU.ID).First(&card)
	var order Order
	app.db.First(&order, result.Order.ID)
	if card.Status != "available" || card.ReservedOrderID != nil || order.Status != "expired" {
		t.Fatalf("card=%+v order=%+v", card, order)
	}
}

func TestConcurrentBuyersCannotReceiveTheSameCard(t *testing.T) {
	app := testApp(t)
	first := seedFixture(t, app, 1000, "LAST-CARD")
	hash, _ := passwordHash("password123")
	second := User{Username: randomID("user"), Email: randomID("mail") + "@example.test", PasswordHash: hash, Role: "user", Status: "active", BalanceCents: 1000}
	if err := app.db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	users := []User{first.User, second}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for index := range users {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, errs[i] = app.createOrder(&users[i], createOrderRequest{SKUID: first.SKU.ID, Quantity: 1, PaymentMethod: "balance", IdempotencyKey: "concurrent-idem-00" + string(rune('1'+i))})
		}(index)
	}
	wg.Wait()
	successes, stockFailures := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if err == errInsufficientStock {
			stockFailures++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	var sold, ledgers int64
	app.db.Model(&Card{}).Where("status = ?", "sold").Count(&sold)
	app.db.Model(&BalanceLedger{}).Where("type = ?", "purchase").Count(&ledgers)
	if successes != 1 || stockFailures != 1 || sold != 1 || ledgers != 1 {
		t.Fatalf("success=%d stock_failures=%d sold=%d ledgers=%d", successes, stockFailures, sold, ledgers)
	}
}

func TestAdminOrderListIncludesItemAndBuyer(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 1000, "MEMBER-CARD", "GUEST-LIST-CARD")
	if _, _, err := app.createOrder(&fixture.User, createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 1, PaymentMethod: "balance", IdempotencyKey: "admin-list-member"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.createOrder(nil, createOrderRequest{SKUID: fixture.SKU.ID, Quantity: 1, PaymentMethod: "online", IdempotencyKey: "admin-list-guest", ContactType: "qq", Contact: "12345678", QueryPassword: "query789"}); err != nil {
		t.Fatal(err)
	}
	userToken, err := issueToken(app.config.JWTSecret, fixture.User)
	if err != nil {
		t.Fatal(err)
	}
	memberReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/orders", nil)
	memberReq.Header.Set("Authorization", "Bearer "+userToken)
	memberRes := httptest.NewRecorder()
	app.router.ServeHTTP(memberRes, memberReq)
	if memberRes.Code != http.StatusOK || !bytes.Contains(memberRes.Body.Bytes(), []byte(`"product_name":"测试商品"`)) {
		t.Fatalf("member orders missing item snapshot: status=%d body=%s", memberRes.Code, memberRes.Body.String())
	}
	hash, _ := passwordHash("password123")
	admin := User{Username: randomID("admin"), Email: randomID("admin") + "@example.test", PasswordHash: hash, Role: "admin", Status: "active"}
	if err := app.db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	token, err := issueToken(app.config.JWTSecret, admin)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?q=测试商品", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	app.router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Items []struct {
			Order         Order     `json:"order"`
			Item          OrderItem `json:"item"`
			User          *User     `json:"user"`
			ContactMasked string    `json:"contact_masked"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("items=%d body=%s", len(payload.Items), res.Body.String())
	}
	memberFound, guestFound := false, false
	for _, item := range payload.Items {
		if item.Item.ProductNameSnapshot != "测试商品" || item.Item.SKUNameSnapshot != "标准规格" {
			t.Fatalf("missing item snapshot: %+v", item.Item)
		}
		if item.Order.UserID != nil && item.User != nil && item.User.Username == fixture.User.Username {
			memberFound = true
		}
		if item.Order.UserID == nil && item.ContactMasked == "12****78" {
			guestFound = true
		}
	}
	if !memberFound || !guestFound {
		t.Fatalf("member=%v guest=%v body=%s", memberFound, guestFound, res.Body.String())
	}
}

func TestUnknownAPIReturnsJSONWithoutWebRoot(t *testing.T) {
	app := testApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/refunds", nil)
	res := httptest.NewRecorder()
	app.router.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound || res.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status=%d content-type=%q body=%s", res.Code, res.Header().Get("Content-Type"), res.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil || payload.Error.Code != "not_found" {
		t.Fatalf("invalid JSON error: err=%v body=%s", err, res.Body.String())
	}
}

func TestLoadConfigFileBusinessSwitches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"registration_enabled":false,"guest_checkout_enabled":false,"balance_payment_enabled":false,"online_payment_enabled":false,"show_stock_count":true,"max_purchase_quantity":7,"payment_timeout_minutes":3}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_CONFIG_PATH", path)
	t.Setenv("MAX_PURCHASE_QUANTITY", "")
	t.Setenv("PAYMENT_TIMEOUT_MINUTES", "")
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.RegistrationEnabled || config.GuestCheckoutEnabled || config.BalancePaymentEnabled || config.OnlinePaymentEnabled || !config.ShowStockCount {
		t.Fatalf("switches not applied: %+v", config)
	}
	if config.MaxPurchaseQuantity != 7 || config.PaymentTimeout != 3*time.Minute {
		t.Fatalf("limits not applied: quantity=%d timeout=%s", config.MaxPurchaseQuantity, config.PaymentTimeout)
	}
}

func TestLoadConfigSMTPSettings(t *testing.T) {
	t.Setenv("APP_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "sender@example.test")
	t.Setenv("SMTP_PASSWORD", "test-only-password")
	t.Setenv("SMTP_FROM", "sender@example.test")
	t.Setenv("SMTP_TLS", "true")
	t.Setenv("REGISTRATION_CODE_HASH_KEY", "registration-hash-test")
	t.Setenv("REGISTRATION_CODE_TTL_MINUTES", "12")
	t.Setenv("REGISTRATION_CODE_RESEND_SECONDS", "75")
	t.Setenv("REGISTRATION_CODE_MAX_ATTEMPTS", "4")

	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.SMTPHost != "smtp.example.test" || config.SMTPPort != 465 || config.SMTPUsername != "sender@example.test" ||
		config.SMTPPassword != "test-only-password" || config.SMTPFrom != "sender@example.test" || !config.SMTPTLS {
		t.Fatal("SMTP settings were not loaded from the environment")
	}
	if config.RegistrationCodeHashKey != "registration-hash-test" || config.RegistrationCodeTTL != 12*time.Minute ||
		config.RegistrationCodeResendInterval != 75*time.Second || config.RegistrationCodeMaxAttempts != 4 {
		t.Fatal("registration verification settings were not loaded from the environment")
	}
}

func TestRegistrationRequiresQQEmailAndConsumesHashedCode(t *testing.T) {
	app := testApp(t)
	mailer := &fakeRegistrationMailer{}
	app.mailer = mailer

	response := postJSON(app, "/api/v1/auth/registration-codes", map[string]string{"email": "member@example.com"})
	if response.Code != http.StatusBadRequest || mailer.calls != 0 {
		t.Fatalf("non-QQ email status=%d calls=%d body=%s", response.Code, mailer.calls, response.Body.String())
	}
	response = postJSON(app, "/api/v1/auth/register", map[string]string{
		"username": "nonqqmember", "email": "member@example.com", "password": "password123", "verification_code": "123456",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("non-QQ registration status=%d body=%s", response.Code, response.Body.String())
	}

	response = postJSON(app, "/api/v1/auth/registration-codes", map[string]string{"email": " 123456789@QQ.COM "})
	if response.Code != http.StatusAccepted || mailer.calls != 1 || mailer.recipient != "123456789@qq.com" || !validVerificationCode(mailer.code) {
		t.Fatalf("send status=%d calls=%d recipient=%q body=%s", response.Code, mailer.calls, mailer.recipient, response.Body.String())
	}
	var verification EmailVerification
	if err := app.db.Where("email = ?", mailer.recipient).First(&verification).Error; err != nil {
		t.Fatal(err)
	}
	if verification.CodeHash == "" || verification.CodeHash == mailer.code || verification.CodeHash != app.registrationCodeHash(mailer.recipient, mailer.code) {
		t.Fatal("verification code was not stored as the expected keyed hash")
	}

	registerBody := map[string]string{
		"username": "member001", "email": "123456789@qq.com", "password": "password123", "verification_code": mailer.code,
	}
	response = postJSON(app, "/api/v1/auth/register", registerBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", response.Code, response.Body.String())
	}
	if err := app.db.First(&verification, verification.ID).Error; err != nil || verification.ConsumedAt == nil {
		t.Fatalf("verification was not consumed: err=%v row=%+v", err, verification)
	}

	registerBody["username"] = "member002"
	response = postJSON(app, "/api/v1/auth/register", registerBody)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("reused code status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRegistrationRejectsWrongAndExpiredCodes(t *testing.T) {
	app := testApp(t)
	mailer := &fakeRegistrationMailer{}
	app.mailer = mailer
	email := "987654321@qq.com"

	response := postJSON(app, "/api/v1/auth/registration-codes", map[string]string{"email": email})
	if response.Code != http.StatusAccepted {
		t.Fatalf("send status=%d body=%s", response.Code, response.Body.String())
	}
	wrongCode := "000000"
	if mailer.code == wrongCode {
		wrongCode = "000001"
	}
	response = postJSON(app, "/api/v1/auth/register", map[string]string{
		"username": "wrongcode", "email": email, "password": "password123", "verification_code": wrongCode,
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("wrong code status=%d body=%s", response.Code, response.Body.String())
	}
	var verification EmailVerification
	if err := app.db.Where("email = ?", email).First(&verification).Error; err != nil {
		t.Fatal(err)
	}
	if verification.Attempts != 1 {
		t.Fatalf("attempts=%d, want 1", verification.Attempts)
	}
	if err := app.db.Model(&verification).Update("expires_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	response = postJSON(app, "/api/v1/auth/register", map[string]string{
		"username": "expiredcode", "email": email, "password": "password123", "verification_code": mailer.code,
	})
	if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("verification_code_expired")) {
		t.Fatalf("expired code status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRegistrationCodeLocksAfterMaximumAttempts(t *testing.T) {
	app := testApp(t)
	app.config.RegistrationCodeMaxAttempts = 2
	mailer := &fakeRegistrationMailer{}
	app.mailer = mailer
	email := "6677889900@qq.com"

	response := postJSON(app, "/api/v1/auth/registration-codes", map[string]string{"email": email})
	if response.Code != http.StatusAccepted {
		t.Fatalf("send status=%d body=%s", response.Code, response.Body.String())
	}
	wrongCode := "000000"
	if mailer.code == wrongCode {
		wrongCode = "000001"
	}
	for attempt := 0; attempt < app.config.RegistrationCodeMaxAttempts; attempt++ {
		response = postJSON(app, "/api/v1/auth/register", map[string]string{
			"username": "lockedcode", "email": email, "password": "password123", "verification_code": wrongCode,
		})
		if response.Code != http.StatusBadRequest {
			t.Fatalf("wrong code attempt %d status=%d body=%s", attempt+1, response.Code, response.Body.String())
		}
	}
	response = postJSON(app, "/api/v1/auth/register", map[string]string{
		"username": "lockedcode", "email": email, "password": "password123", "verification_code": mailer.code,
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("locked correct code status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRegistrationCodeSendIsThrottledAndSMTPFailureLeavesNoCode(t *testing.T) {
	app := testApp(t)
	mailer := &fakeRegistrationMailer{}
	app.mailer = mailer
	email := "1122334455@qq.com"

	response := postJSON(app, "/api/v1/auth/registration-codes", map[string]string{"email": email})
	if response.Code != http.StatusAccepted {
		t.Fatalf("first send status=%d body=%s", response.Code, response.Body.String())
	}
	response = postJSON(app, "/api/v1/auth/registration-codes", map[string]string{"email": email})
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" || mailer.calls != 1 {
		t.Fatalf("throttled send status=%d calls=%d retry=%q body=%s", response.Code, mailer.calls, response.Header().Get("Retry-After"), response.Body.String())
	}

	failedApp := testApp(t)
	failedMailer := &fakeRegistrationMailer{err: errors.New("simulated delivery failure")}
	failedApp.mailer = failedMailer
	failedEmail := "5566778899@qq.com"
	response = postJSON(failedApp, "/api/v1/auth/registration-codes", map[string]string{"email": failedEmail})
	if response.Code != http.StatusBadGateway {
		t.Fatalf("failed send status=%d body=%s", response.Code, response.Body.String())
	}
	var count int64
	if err := failedApp.db.Model(&EmailVerification{}).Where("email = ?", failedEmail).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("SMTP failure left %d verification rows", count)
	}
}

func TestAdminExportAndAllocateSKUCards(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "EXPORT-A", "EXPORT-B", "EXPORT-C")
	hash, _ := passwordHash("password123")
	admin := User{Username: randomID("admin"), Email: randomID("admin") + "@example.test", PasswordHash: hash, Role: "admin", Status: "active"}
	if err := app.db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	token, err := issueToken(app.config.JWTSecret, admin)
	if err != nil {
		t.Fatal(err)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/skus/"+itoa(fixture.SKU.ID)+"/cards/export?limit=2", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	app.router.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", exportRes.Code, exportRes.Body.String())
	}
	var exportPayload struct {
		Count   int      `json:"count"`
		Secrets []string `json:"secrets"`
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	if exportPayload.Count != 2 || len(exportPayload.Secrets) != 2 {
		t.Fatalf("export payload=%+v", exportPayload)
	}
	var stillAvailable int64
	app.db.Model(&Card{}).Where("sku_id = ? AND status = ?", fixture.SKU.ID, "available").Count(&stillAvailable)
	if stillAvailable != 3 {
		t.Fatalf("export should not change status, available=%d", stillAvailable)
	}

	allocateRes := postJSONAuth(app, http.MethodPost, "/api/v1/admin/skus/"+itoa(fixture.SKU.ID)+"/cards/allocate", map[string]any{"count": 2, "mark_as": "allocated"}, token)
	if allocateRes.Code != http.StatusOK {
		t.Fatalf("allocate status=%d body=%s", allocateRes.Code, allocateRes.Body.String())
	}
	var allocatePayload struct {
		Count   int      `json:"count"`
		Status  string   `json:"status"`
		Secrets []string `json:"secrets"`
	}
	if err := json.Unmarshal(allocateRes.Body.Bytes(), &allocatePayload); err != nil {
		t.Fatal(err)
	}
	if allocatePayload.Count != 2 || allocatePayload.Status != "allocated" || len(allocatePayload.Secrets) != 2 {
		t.Fatalf("allocate payload=%+v", allocatePayload)
	}
	var allocated, available int64
	app.db.Model(&Card{}).Where("sku_id = ? AND status = ?", fixture.SKU.ID, "allocated").Count(&allocated)
	app.db.Model(&Card{}).Where("sku_id = ? AND status = ?", fixture.SKU.ID, "available").Count(&available)
	if allocated != 2 || available != 1 {
		t.Fatalf("allocated=%d available=%d", allocated, available)
	}
}

func TestPublicTrafficClaimByClaimCode(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "UPSTREAM-SECRET-001")
	claimCode, cardBefore := firstClaimCode(t, app, fixture.SKU.ID)
	if claimCode == "UPSTREAM-SECRET-001" {
		t.Fatal("claim code must differ from upstream secret")
	}

	bySecret := postJSON(app, "/api/v1/public/traffic/claim", map[string]string{"claim_code": "UPSTREAM-SECRET-001"})
	if bySecret.Code != http.StatusNotFound {
		t.Fatalf("secret as claim should 404, got %d %s", bySecret.Code, bySecret.Body.String())
	}

	first := postJSON(app, "/api/v1/public/traffic/claim", map[string]string{
		"claim_code":     claimCode,
		"query_password": "QueryPass1",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first claim status=%d body=%s", first.Code, first.Body.String())
	}
	var payload struct {
		ProductName string   `json:"product_name"`
		SKUName     string   `json:"sku_name"`
		Secrets     []string `json:"secrets"`
		ClaimedAt   string   `json:"claimed_at"`
		Replay      bool     `json:"replay"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ProductName != "测试商品" || payload.SKUName != "标准规格" || len(payload.Secrets) != 1 || payload.Secrets[0] != "UPSTREAM-SECRET-001" || payload.ClaimedAt == "" || payload.Replay {
		t.Fatalf("claim payload=%+v", payload)
	}
	var card Card
	if err := app.db.First(&card, cardBefore.ID).Error; err != nil {
		t.Fatal(err)
	}
	if card.Status != "sold" || card.SoldAt == nil || card.ClaimedAt == nil || card.QueryPasswordHash == "" {
		t.Fatalf("card not marked sold/claimed: %+v", card)
	}

	second := postJSON(app, "/api/v1/public/traffic/claim", map[string]string{"claim_code": claimCode})
	if second.Code != http.StatusConflict || !bytes.Contains(second.Body.Bytes(), []byte("claim_already_used")) {
		t.Fatalf("second claim status=%d body=%s", second.Code, second.Body.String())
	}
	if bytes.Contains(second.Body.Bytes(), []byte("UPSTREAM-SECRET-001")) {
		t.Fatalf("used claim must not re-reveal secret: %s", second.Body.String())
	}

	replay := postJSON(app, "/api/v1/public/traffic/claim", map[string]string{
		"claim_code":     claimCode,
		"query_password": "QueryPass1",
	})
	if replay.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	var replayPayload struct {
		Secrets []string `json:"secrets"`
		Replay  bool     `json:"replay"`
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &replayPayload); err != nil {
		t.Fatal(err)
	}
	if !replayPayload.Replay || len(replayPayload.Secrets) != 1 || replayPayload.Secrets[0] != "UPSTREAM-SECRET-001" {
		t.Fatalf("replay payload=%+v", replayPayload)
	}

	missing := postJSON(app, "/api/v1/public/traffic/claim", map[string]string{"claim_code": "NOT-EXIST"})
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing claim status=%d body=%s", missing.Code, missing.Body.String())
	}
}

func TestAdminExportReturnsClaimCodesNotSecrets(t *testing.T) {
	app := testApp(t)
	fixture := seedFixture(t, app, 0, "REAL-SECRET-A", "REAL-SECRET-B")
	hash, _ := passwordHash("password123")
	admin := User{Username: randomID("admin"), Email: randomID("admin") + "@example.test", PasswordHash: hash, Role: "admin", Status: "active"}
	if err := app.db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	token, err := issueToken(app.config.JWTSecret, admin)
	if err != nil {
		t.Fatal(err)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/skus/"+itoa(fixture.SKU.ID)+"/cards/export?limit=2", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	app.router.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", exportRes.Code, exportRes.Body.String())
	}
	var exportPayload struct {
		Count      int      `json:"count"`
		Secrets    []string `json:"secrets"`
		ClaimCodes []string `json:"claim_codes"`
		Items      []struct {
			ClaimCode     string `json:"claim_code"`
			QueryPassword string `json:"query_password"`
			Secret        string `json:"secret"`
		} `json:"items"`
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	if exportPayload.Count != 2 || len(exportPayload.ClaimCodes) != 2 || len(exportPayload.Items) != 2 {
		t.Fatalf("export payload=%+v", exportPayload)
	}
	for _, item := range exportPayload.Items {
		if item.ClaimCode == "" || !strings.HasPrefix(item.ClaimCode, "TRAF-") {
			t.Fatalf("expected TRAF claim code, got %+v", item)
		}
		if item.Secret != "" {
			t.Fatalf("default export must not include real secret: %+v", item)
		}
		if item.QueryPassword == "" {
			t.Fatalf("export should include query password once: %+v", item)
		}
		if item.ClaimCode == "REAL-SECRET-A" || item.ClaimCode == "REAL-SECRET-B" {
			t.Fatalf("claim code leaked real secret")
		}
	}
	for _, s := range exportPayload.Secrets {
		if s == "REAL-SECRET-A" || s == "REAL-SECRET-B" {
			t.Fatalf("legacy secrets field still has real secret: %v", exportPayload.Secrets)
		}
	}

	allocateRes := postJSONAuth(app, http.MethodPost, "/api/v1/admin/skus/"+itoa(fixture.SKU.ID)+"/cards/allocate", map[string]any{"count": 1, "mark_as": "allocated"}, token)
	if allocateRes.Code != http.StatusOK {
		t.Fatalf("allocate status=%d body=%s", allocateRes.Code, allocateRes.Body.String())
	}
	var allocatePayload struct {
		Items []struct {
			ClaimCode string `json:"claim_code"`
			Status    string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(allocateRes.Body.Bytes(), &allocatePayload); err != nil {
		t.Fatal(err)
	}
	if len(allocatePayload.Items) != 1 || allocatePayload.Items[0].Status != "allocated" {
		t.Fatalf("allocate payload=%+v", allocatePayload)
	}
	claim := postJSON(app, "/api/v1/public/traffic/claim", map[string]string{"claim_code": allocatePayload.Items[0].ClaimCode})
	if claim.Code != http.StatusOK {
		t.Fatalf("allocated card claim status=%d body=%s", claim.Code, claim.Body.String())
	}
	if !bytes.Contains(claim.Body.Bytes(), []byte("REAL-SECRET-")) {
		t.Fatalf("claim should reveal real secret: %s", claim.Body.String())
	}
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func postJSON(app *App, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	app.router.ServeHTTP(res, req)
	return res
}

func postJSONAuth(app *App, method, path string, body any, token string) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	app.router.ServeHTTP(res, req)
	return res
}

func postSignedCallback(app *App, body paymentCallbackRequest, signature string) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/payments/mock", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Payment-Signature", signature)
	res := httptest.NewRecorder()
	app.router.ServeHTTP(res, req)
	return res
}


func TestResetAdminPasswordFromEnv(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reset-admin.db")
	config := Config{
		Env: "test", DatabasePath: dbPath, BaseURL: "http://example.test", JWTSecret: "jwt-test-key",
		CardEncryptKey: "card-encrypt-test", CardHashKey: "card-hash-test", ContactEncryptKey: "contact-encrypt-test", ContactHashKey: "contact-hash-test",
		PaymentProvider: "mock", PaymentMerchantID: "mock-merchant", PaymentMerchantKey: "payment-test-key", PaymentTimeout: time.Minute, MaxPurchaseQuantity: 100,
		RegistrationEnabled: true, SMTPHost: "smtp.example.test", SMTPPort: 465, SMTPUsername: "sender@example.test", SMTPPassword: "test-only-password",
		SMTPFrom: "sender@example.test", SMTPTLS: true, RegistrationCodeHashKey: "registration-code-test-key", RegistrationCodeTTL: 10 * time.Minute,
		RegistrationCodeResendInterval: time.Minute, RegistrationCodeMaxAttempts: 5,
		BootstrapAdminEmail: "admin@example.com", BootstrapAdminPassword: "old-password-should-not-login",
	}
	app, err := newApp(config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := app.db.DB()
	_ = sqlDB.Close()

	// 启动时带 RESET_ADMIN_PASSWORD 应覆盖已有 admin 密码
	config.ResetAdminPassword = "NewAdminPass123"
	config.ResetAdminEmail = "admin@example.com"
	config.BootstrapAdminPassword = "" // 已有 admin，bootstrap 不会再生
	app2, err := newApp(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := app2.db.DB()
		_ = sqlDB.Close()
	})

	var admin User
	if err := app2.db.Where("email = ?", "admin@example.com").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if !passwordMatches(admin.PasswordHash, "NewAdminPass123") {
		t.Fatal("admin password was not reset from env")
	}
	if passwordMatches(admin.PasswordHash, "old-password-should-not-login") {
		t.Fatal("old password should no longer match")
	}

	// 登录接口应接受新密码
	res := postJSON(app2, "/api/v1/auth/login", map[string]string{
		"login":    "admin@example.com",
		"password": "NewAdminPass123",
	})
	if res.Code != http.StatusOK {
		t.Fatalf("login after reset status=%d body=%s", res.Code, res.Body.String())
	}
}


func TestPublicTrafficClaimCORS(t *testing.T) {
	app := testApp(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/public/traffic/claim", nil)
	req.Header.Set("Origin", "https://card.xiaoasi.xyz")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	res := httptest.NewRecorder()
	app.router.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("preflight status=%d body=%s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "https://card.xiaoasi.xyz" {
		t.Fatalf("Allow-Origin=%q", got)
	}

	// 非公开路径不应被 CORS 放行
	adminReq := httptest.NewRequest(http.MethodOptions, "/api/v1/admin/orders", nil)
	adminReq.Header.Set("Origin", "https://card.xiaoasi.xyz")
	adminRes := httptest.NewRecorder()
	app.router.ServeHTTP(adminRes, adminReq)
	if adminRes.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("admin path should not expose CORS origin")
	}
}
