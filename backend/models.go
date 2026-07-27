package main

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Email        string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:16;not null;default:user" json:"role"`
	Status       string    `gorm:"size:16;not null;default:active" json:"status"`
	BalanceCents int64     `gorm:"not null;default:0;check:balance_cents >= 0" json:"balance_cents"`
	Version      uint      `gorm:"not null;default:0" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type EmailVerification struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Email      string     `gorm:"size:255;not null;index:idx_email_verification_lookup,priority:1" json:"email"`
	Purpose    string     `gorm:"size:32;not null;index:idx_email_verification_lookup,priority:2" json:"purpose"`
	CodeHash   string     `gorm:"size:64;not null" json:"-"`
	ExpiresAt  time.Time  `gorm:"not null;index" json:"expires_at"`
	SentAt     time.Time  `gorm:"not null;index:idx_email_verification_lookup,priority:3,sort:desc" json:"sent_at"`
	Attempts   int        `gorm:"not null;default:0" json:"-"`
	ConsumedAt *time.Time `gorm:"index" json:"consumed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type BalanceLedger struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	UserID            uint      `gorm:"not null;uniqueIndex:idx_ledger_idem" json:"user_id"`
	Type              string    `gorm:"size:32;not null" json:"type"`
	Direction         string    `gorm:"size:8;not null" json:"direction"`
	AmountCents       int64     `gorm:"not null;check:amount_cents > 0" json:"amount_cents"`
	BalanceAfterCents int64     `gorm:"not null" json:"balance_after_cents"`
	RefType           string    `gorm:"size:32" json:"ref_type"`
	RefID             uint      `json:"ref_id"`
	Reason            string    `gorm:"size:500" json:"reason"`
	OperatorID        *uint     `json:"operator_id"`
	IdempotencyKey    string    `gorm:"size:100;not null;uniqueIndex:idx_ledger_idem" json:"idempotency_key"`
	CreatedAt         time.Time `json:"created_at"`
}

type Platform struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Website   string    `gorm:"size:500" json:"website"`
	Status    string    `gorm:"size:16;not null;default:active;index" json:"status"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PlatformID  uint      `gorm:"not null;index" json:"platform_id"`
	Platform    Platform  `json:"platform,omitempty"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Slug        string    `gorm:"size:200;uniqueIndex;not null" json:"slug"`
	Description string    `gorm:"type:text" json:"description"`
	CoverURL    string    `gorm:"size:1000" json:"cover_url"`
	Status      string    `gorm:"size:16;not null;default:active;index" json:"status"`
	Sort        int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SKU struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ProductID      uint      `gorm:"not null;index" json:"product_id"`
	Product        Product   `json:"product,omitempty"`
	Name           string    `gorm:"size:200;not null" json:"name"`
	AttrsJSON      string    `gorm:"type:text" json:"attrs_json"`
	SalePriceCents int64     `gorm:"not null;check:sale_price_cents > 0" json:"sale_price_cents"`
	Status         string    `gorm:"size:16;not null;default:active;index" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CardBatch struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	SKUID          uint      `gorm:"column:sku_id;not null;index" json:"sku_id"`
	Filename       string    `gorm:"size:255" json:"filename"`
	TotalCount     int       `gorm:"not null" json:"total_count"`
	SuccessCount   int       `gorm:"not null" json:"success_count"`
	DuplicateCount int       `gorm:"not null" json:"duplicate_count"`
	InvalidCount   int       `gorm:"not null" json:"invalid_count"`
	ImportedBy     uint      `gorm:"not null" json:"imported_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type Card struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	SKUID            uint       `gorm:"column:sku_id;not null;index:idx_card_allocate,priority:1;index" json:"sku_id"`
	BatchID          uint       `gorm:"not null;index" json:"batch_id"`
	SecretCiphertext string     `gorm:"type:text;not null" json:"-"`
	SecretHash       string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	KeyVersion       int        `gorm:"not null;default:1" json:"key_version"`
	Status           string     `gorm:"size:16;not null;index:idx_card_allocate,priority:2;index:idx_card_expire,priority:1" json:"status"`
	ReservedOrderID  *uint      `gorm:"index" json:"reserved_order_id"`
	ReservedUntil    *time.Time `gorm:"index:idx_card_expire,priority:2" json:"reserved_until"`
	SoldOrderID      *uint      `gorm:"index" json:"sold_order_id"`
	SoldAt           *time.Time `json:"sold_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Order struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	OrderNo           string     `gorm:"size:40;uniqueIndex;not null" json:"order_no"`
	UserID            *uint      `gorm:"index" json:"user_id,omitempty"`
	PaymentMethod     string     `gorm:"size:16;not null" json:"payment_method"`
	ContactType       string     `gorm:"size:16" json:"contact_type,omitempty"`
	ContactCiphertext string     `gorm:"type:text" json:"-"`
	ContactHash       string     `gorm:"size:64;index:idx_guest_query,priority:1" json:"-"`
	QueryPasswordHash string     `gorm:"size:255" json:"-"`
	Status            string     `gorm:"size:32;not null;index:idx_order_expire,priority:1" json:"status"`
	Currency          string     `gorm:"size:3;not null;default:CNY" json:"currency"`
	TotalAmountCents  int64      `gorm:"not null;check:total_amount_cents > 0" json:"total_amount_cents"`
	IdempotencyKey    string     `gorm:"size:100;uniqueIndex;not null" json:"-"`
	ExpiresAt         *time.Time `gorm:"index:idx_order_expire,priority:2" json:"expires_at,omitempty"`
	CreatedAt         time.Time  `gorm:"index" json:"created_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

type OrderItem struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	OrderID             uint   `gorm:"not null;uniqueIndex" json:"order_id"`
	SKUID               uint   `gorm:"column:sku_id;not null;index" json:"sku_id"`
	ProductNameSnapshot string `gorm:"size:200;not null" json:"product_name"`
	SKUNameSnapshot     string `gorm:"size:200;not null" json:"sku_name"`
	UnitPriceCents      int64  `gorm:"not null" json:"unit_price_cents"`
	Quantity            int    `gorm:"not null;check:quantity > 0" json:"quantity"`
	SubtotalCents       int64  `gorm:"not null" json:"subtotal_cents"`
}

type OrderCard struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	OrderID     uint `gorm:"not null;index" json:"order_id"`
	OrderItemID uint `gorm:"not null;index" json:"order_item_id"`
	CardID      uint `gorm:"not null;uniqueIndex" json:"card_id"`
}

type Payment struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	OrderID           uint       `gorm:"not null;uniqueIndex" json:"order_id"`
	Provider          string     `gorm:"size:64;not null" json:"provider"`
	MerchantOrderNo   string     `gorm:"size:64;uniqueIndex;not null" json:"merchant_order_no"`
	ProviderTradeNo   string     `gorm:"size:128" json:"provider_trade_no,omitempty"`
	Currency          string     `gorm:"size:3;not null" json:"currency"`
	AmountCents       int64      `gorm:"not null" json:"amount_cents"`
	Status            string     `gorm:"size:16;not null" json:"status"`
	PayURL            string     `gorm:"size:1000" json:"pay_url"`
	CallbackEventHash string     `gorm:"size:64;index" json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CallbackAt        *time.Time `json:"callback_at,omitempty"`
}

type OrderEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderID    uint      `gorm:"not null;index" json:"order_id"`
	FromStatus string    `gorm:"size:32" json:"from_status"`
	ToStatus   string    `gorm:"size:32;not null" json:"to_status"`
	EventType  string    `gorm:"size:64;not null" json:"event_type"`
	RefID      string    `gorm:"size:128" json:"ref_id"`
	DetailJSON string    `gorm:"type:text" json:"detail_json,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OperatorID uint      `gorm:"not null;index" json:"operator_id"`
	Action     string    `gorm:"size:64;not null" json:"action"`
	TargetType string    `gorm:"size:64;not null" json:"target_type"`
	TargetID   string    `gorm:"size:64" json:"target_id"`
	BeforeJSON string    `gorm:"type:text" json:"before_json,omitempty"`
	AfterJSON  string    `gorm:"type:text" json:"after_json,omitempty"`
	Reason     string    `gorm:"size:500" json:"reason,omitempty"`
	IP         string    `gorm:"size:64" json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}
