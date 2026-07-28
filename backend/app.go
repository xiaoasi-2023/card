package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type App struct {
	config         Config
	db             *gorm.DB
	router         *gin.Engine
	writeMu        sync.Mutex
	verificationMu sync.Mutex
	claimRateMu    sync.Mutex
	claimHits      map[string][]time.Time
	mailer         registrationMailer
}

func newApp(config Config) (*App, error) {
	if config.DatabasePath != ":memory:" && !strings.HasPrefix(config.DatabasePath, "file:") {
		if err := os.MkdirAll(filepath.Dir(config.DatabasePath), 0o750); err != nil {
			return nil, err
		}
	}
	dsn := config.DatabasePath
	if dsn == ":memory:" {
		dsn = "file:card_test?mode=memory&cache=shared"
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if err := db.Exec(pragma).Error; err != nil {
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if err := db.AutoMigrate(&User{}, &EmailVerification{}, &BalanceLedger{}, &Platform{}, &Product{}, &SKU{}, &CardBatch{}, &Card{}, &Order{}, &OrderItem{}, &OrderCard{}, &Payment{}, &OrderEvent{}, &AuditLog{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_trade_no ON payments(provider_trade_no) WHERE provider_trade_no <> ''").Error; err != nil {
		return nil, fmt.Errorf("create payment index: %w", err)
	}
	app := &App{config: config, db: db, mailer: newSMTPMailer(config), claimHits: map[string][]time.Time{}}
	if err := app.backfillClaimCodes(); err != nil {
		return nil, fmt.Errorf("backfill claim codes: %w", err)
	}
	// 领取码 / 哈希部分唯一索引：空值可共存，回填后非空值唯一。
	for _, stmt := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_cards_claim_code_unique ON cards(claim_code) WHERE claim_code <> ''",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_cards_claim_code_hash_unique ON cards(claim_code_hash) WHERE claim_code_hash <> ''",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, fmt.Errorf("create claim index: %w", err)
		}
	}
	if err := app.bootstrapAdmin(); err != nil {
		return nil, err
	}
	if err := app.resetAdminPasswordFromEnv(); err != nil {
		return nil, err
	}
	app.router = app.routes()
	if _, err := app.releaseExpired(time.Now()); err != nil {
		return nil, err
	}
	return app, nil
}

func (a *App) bootstrapAdmin() error {
	if a.config.BootstrapAdminEmail == "" || a.config.BootstrapAdminPassword == "" {
		return nil
	}
	var count int64
	if err := a.db.Model(&User{}).Where("role = ?", "admin").Count(&count).Error; err != nil || count > 0 {
		return err
	}
	hash, err := passwordHash(a.config.BootstrapAdminPassword)
	if err != nil {
		return err
	}
	name := strings.Split(a.config.BootstrapAdminEmail, "@")[0]
	return a.db.Create(&User{Username: name, Email: strings.ToLower(a.config.BootstrapAdminEmail), PasswordHash: hash, Role: "admin", Status: "active"}).Error
}
// resetAdminPasswordFromEnv 根据 RESET_ADMIN_PASSWORD 在启动时重置管理员密码。
// 这是运维应急手段，不是公开 URL；用完必须清掉环境变量再重启。
func (a *App) resetAdminPasswordFromEnv() error {
	plain := strings.TrimSpace(a.config.ResetAdminPassword)
	if plain == "" {
		return nil
	}
	if len(plain) < 8 {
		return fmt.Errorf("RESET_ADMIN_PASSWORD 至少 8 位")
	}
	hash, err := passwordHash(plain)
	if err != nil {
		return fmt.Errorf("hash RESET_ADMIN_PASSWORD: %w", err)
	}

	query := a.db.Model(&User{}).Where("role = ?", "admin")
	targetEmail := strings.ToLower(strings.TrimSpace(a.config.ResetAdminEmail))
	if targetEmail != "" {
		query = query.Where("LOWER(email) = ?", targetEmail)
	}

	var admins []User
	if err := query.Find(&admins).Error; err != nil {
		return fmt.Errorf("query admin for reset: %w", err)
	}
	if len(admins) == 0 {
		if targetEmail != "" {
			return fmt.Errorf("RESET_ADMIN_PASSWORD 已设置，但找不到 admin 邮箱 %s", targetEmail)
		}
		return fmt.Errorf("RESET_ADMIN_PASSWORD 已设置，但库中没有 admin 用户；可先配 BOOTSTRAP_ADMIN_* 创建")
	}

	now := time.Now()
	ids := make([]uint, 0, len(admins))
	emails := make([]string, 0, len(admins))
	for _, admin := range admins {
		ids = append(ids, admin.ID)
		emails = append(emails, admin.Email)
	}
	result := a.db.Model(&User{}).Where("id IN ? AND role = ?", ids, "admin").Updates(map[string]any{
		"password_hash": hash,
		"updated_at":    now,
		"status":        "active",
	})
	if result.Error != nil {
		return fmt.Errorf("reset admin password: %w", result.Error)
	}
	if result.RowsAffected < 1 {
		return fmt.Errorf("reset admin password: no rows updated")
	}

	detail, _ := json.Marshal(map[string]any{
		"admin_ids": ids,
		"emails":    emails,
		"source":    "RESET_ADMIN_PASSWORD",
	})
	_ = a.db.Create(&AuditLog{
		OperatorID: 0,
		Action:     "admin.password_reset_env",
		TargetType: "user",
		TargetID:   fmt.Sprint(ids[0]),
		AfterJSON:  string(detail),
		Reason:     "startup reset via RESET_ADMIN_PASSWORD",
		IP:         "startup",
	}).Error

	log.Printf(
		"security: reset %d admin password(s) via RESET_ADMIN_PASSWORD (emails=%s). Clear RESET_ADMIN_PASSWORD and restart after login.",
		result.RowsAffected,
		strings.Join(emails, ","),
	)
	return nil
}


func (a *App) routes() *gin.Engine {
	if a.config.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), requestIDMiddleware(), corsPublicMiddleware())
	r.GET("/healthz", func(c *gin.Context) {
		sqlDB, err := a.db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			fail(c, http.StatusServiceUnavailable, "database_unavailable", "数据库不可用")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	pub := v1.Group("/public")
	pub.GET("/platforms", a.listPublicPlatforms)
	pub.GET("/products", a.listPublicProducts)
	pub.GET("/products/:slug", a.getPublicProduct)
	pub.POST("/traffic/claim", a.publicTrafficClaim)

	auth := v1.Group("/auth")
	auth.POST("/registration-codes", a.sendRegistrationCode)
	auth.POST("/register", a.register)
	auth.POST("/login", a.login)
	auth.POST("/logout", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	me := v1.Group("/me", a.authRequired(false))
	me.GET("", a.getMe)
	me.PUT("/password", a.changePassword)
	me.GET("/balance-ledgers", a.listMyLedgers)
	me.POST("/orders", a.createMemberOrder)
	me.GET("/orders", a.listMyOrders)
	me.GET("/orders/:order_no", a.getMyOrder)

	guest := v1.Group("/guest")
	guest.POST("/orders", a.createGuestOrder)
	guest.POST("/orders/query", a.queryGuestOrder)
	v1.POST("/webhooks/payments/:provider", a.paymentWebhook)
	if a.config.Env != "production" && a.config.PaymentProvider == "mock" {
		v1.POST("/dev/payments/:order_no/pay", a.mockPay)
	}

	admin := v1.Group("/admin", a.authRequired(true))
	admin.GET("/users", a.adminListUsers)
	admin.POST("/users", a.adminCreateUser)
	admin.PUT("/users/:id", a.adminUpdateUser)
	admin.POST("/users/:id/balance-adjustments", a.adminAdjustBalance)
	admin.GET("/platforms", a.adminListPlatforms)
	admin.POST("/platforms", a.adminCreatePlatform)
	admin.PUT("/platforms/:id", a.adminUpdatePlatform)
	admin.DELETE("/platforms/:id", a.adminDeletePlatform)
	admin.GET("/products", a.adminListProducts)
	admin.POST("/products", a.adminCreateProduct)
	admin.PUT("/products/:id", a.adminUpdateProduct)
	admin.DELETE("/products/:id", a.adminDeleteProduct)
	admin.GET("/skus", a.adminListSKUs)
	admin.POST("/skus", a.adminCreateSKU)
	admin.PUT("/skus/:id", a.adminUpdateSKU)
	admin.DELETE("/skus/:id", a.adminDeleteSKU)
	admin.POST("/card-batches", a.adminImportCards)
	admin.GET("/card-batches/:id", a.adminGetBatch)
	admin.GET("/cards", a.adminListCards)
	admin.PUT("/cards/:id", a.adminUpdateCard)
	admin.GET("/skus/:id/cards/export", a.adminExportSKUCards)
	admin.POST("/skus/:id/cards/allocate", a.adminAllocateSKUCards)
	admin.GET("/orders", a.adminListOrders)
	admin.GET("/orders/:order_no", a.adminGetOrder)
	admin.GET("/payments", a.adminListPayments)
	admin.GET("/audit-logs", a.adminListAuditLogs)

	a.attachStatic(r)
	return r
}

func (a *App) attachStatic(r *gin.Engine) {
	root := a.config.WebRoot
	index := filepath.Join(root, "index.html")
	hasWeb := false
	if info, err := os.Stat(index); err == nil && !info.IsDir() {
		hasWeb = true
		r.Static("/assets", filepath.Join(root, "assets"))
	}
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			fail(c, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		if !hasWeb {
			fail(c, http.StatusNotFound, "not_found", "页面不存在")
			return
		}
		cleanPath := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
		candidate := filepath.Join(root, filepath.FromSlash(cleanPath))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			c.File(candidate)
			return
		}
		c.File(index)
	})
}

func (a *App) authRequired(admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		id, role, err := parseToken(a.config.JWTSecret, raw)
		if err != nil || (admin && role != "admin") {
			fail(c, http.StatusUnauthorized, "unauthorized", "请先登录")
			c.Abort()
			return
		}
		var user User
		if err := a.db.First(&user, id).Error; err != nil || user.Status != "active" || user.Role != role {
			fail(c, http.StatusUnauthorized, "unauthorized", "账号不可用")
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

// corsPublicMiddleware 为跨域公开站调用放行。
// 与图像站公开柜面类似：浏览器从 card.xiaoasi.xyz 直连本服务 API。
// 仅对 /api/v1/public/ 前缀生效，管理端/登录等仍保持同站或显式鉴权。
func corsPublicMiddleware() gin.HandlerFunc {
	allowed := map[string]struct{}{
		"https://card.xiaoasi.xyz":     {},
		"http://card.xiaoasi.xyz":      {},
		"https://www.card.xiaoasi.xyz": {},
		"http://127.0.0.1:5179":        {},
		"http://localhost:5179":        {},
		"http://127.0.0.1:8780":        {},
		"http://localhost:8780":        {},
	}
	// 可用环境变量追加：PUBLIC_CORS_ORIGINS=https://a.com,https://b.com
	if extra := strings.TrimSpace(os.Getenv("PUBLIC_CORS_ORIGINS")); extra != "" {
		for _, item := range strings.Split(extra, ",") {
			origin := strings.TrimSpace(item)
			if origin != "" {
				allowed[origin] = struct{}{}
			}
		}
	}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/v1/public/") {
			c.Next()
			return
		}
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
				c.Header("Access-Control-Max-Age", "86400")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = randomID("req")
		}
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func randomID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func currentUser(c *gin.Context) User {
	value, _ := c.Get("user")
	return value.(User)
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		fail(c, http.StatusBadRequest, "invalid_request", "请求参数不正确")
		return false
	}
	return true
}

func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func paginate(c *gin.Context, query *gorm.DB, out any) error {
	page, pageSize := pagination(c)
	return query.Offset((page - 1) * pageSize).Limit(pageSize).Find(out).Error
}

func (a *App) runExpiryWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if count, err := a.releaseExpired(now); err != nil {
				log.Printf("release expired orders: %v", err)
			} else if count > 0 {
				log.Printf("released %d expired orders", count)
			}
		}
	}
}

func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
