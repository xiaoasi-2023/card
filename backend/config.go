package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                            string
	Addr                           string
	DatabasePath                   string
	BaseURL                        string
	WebRoot                        string
	JWTSecret                      string
	CardEncryptKey                 string
	CardHashKey                    string
	ContactEncryptKey              string
	ContactHashKey                 string
	PaymentProvider                string
	PaymentMerchantID              string
	PaymentMerchantKey             string
	PaymentTimeout                 time.Duration
	MaxPurchaseQuantity            int
	RegistrationEnabled            bool
	GuestCheckoutEnabled           bool
	BalancePaymentEnabled          bool
	OnlinePaymentEnabled           bool
	ShowStockCount                 bool
	BootstrapAdminEmail            string
	BootstrapAdminPassword         string
	SMTPHost                       string
	SMTPPort                       int
	SMTPUsername                   string
	SMTPPassword                   string
	SMTPFrom                       string
	SMTPTLS                        bool
	RegistrationCodeHashKey        string
	RegistrationCodeTTL            time.Duration
	RegistrationCodeResendInterval time.Duration
	RegistrationCodeMaxAttempts    int
}

type fileConfig struct {
	RegistrationEnabled   *bool `json:"registration_enabled"`
	GuestCheckoutEnabled  *bool `json:"guest_checkout_enabled"`
	BalancePaymentEnabled *bool `json:"balance_payment_enabled"`
	OnlinePaymentEnabled  *bool `json:"online_payment_enabled"`
	ShowStockCount        *bool `json:"show_stock_count"`
	MaxPurchaseQuantity   int   `json:"max_purchase_quantity"`
	PaymentTimeoutMinutes int   `json:"payment_timeout_minutes"`
}

func loadConfig() (Config, error) {
	minutes := envInt("PAYMENT_TIMEOUT_MINUTES", 15)
	jwtSecret := env("JWT_SECRET", "dev-jwt-secret-change-me")
	config := Config{
		Env:                            env("APP_ENV", "development"),
		Addr:                           env("APP_ADDR", ":3000"),
		DatabasePath:                   env("DATABASE_PATH", "./data/card.db"),
		BaseURL:                        strings.TrimRight(env("APP_BASE_URL", "http://localhost:3000"), "/"),
		WebRoot:                        env("WEB_ROOT", "./web"),
		JWTSecret:                      jwtSecret,
		CardEncryptKey:                 env("CARD_ENCRYPT_KEY", "dev-card-encrypt-key"),
		CardHashKey:                    env("CARD_HASH_KEY", "dev-card-hash-key"),
		ContactEncryptKey:              env("CONTACT_ENCRYPT_KEY", "dev-contact-encrypt-key"),
		ContactHashKey:                 env("CONTACT_HASH_KEY", "dev-contact-hash-key"),
		PaymentProvider:                env("PAYMENT_PROVIDER", "mock"),
		PaymentMerchantID:              env("PAYMENT_MERCHANT_ID", "mock-merchant"),
		PaymentMerchantKey:             env("PAYMENT_MERCHANT_KEY", "dev-payment-key"),
		PaymentTimeout:                 time.Duration(minutes) * time.Minute,
		MaxPurchaseQuantity:            envInt("MAX_PURCHASE_QUANTITY", 100),
		RegistrationEnabled:            true,
		GuestCheckoutEnabled:           true,
		BalancePaymentEnabled:          true,
		OnlinePaymentEnabled:           true,
		BootstrapAdminEmail:            os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword:         os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		SMTPHost:                       strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:                       envInt("SMTP_PORT", 465),
		SMTPUsername:                   strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:                   os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                       strings.TrimSpace(os.Getenv("SMTP_FROM")),
		SMTPTLS:                        envBool("SMTP_TLS", true),
		RegistrationCodeHashKey:        env("REGISTRATION_CODE_HASH_KEY", jwtSecret),
		RegistrationCodeTTL:            time.Duration(envInt("REGISTRATION_CODE_TTL_MINUTES", 10)) * time.Minute,
		RegistrationCodeResendInterval: time.Duration(envInt("REGISTRATION_CODE_RESEND_SECONDS", 60)) * time.Second,
		RegistrationCodeMaxAttempts:    envInt("REGISTRATION_CODE_MAX_ATTEMPTS", 5),
	}
	path := env("APP_CONFIG_PATH", "./config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return Config{}, fmt.Errorf("read config file: %w", err)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return config, nil
	}
	var file fileConfig
	if err := json.Unmarshal(data, &file); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}
	if file.RegistrationEnabled != nil {
		config.RegistrationEnabled = *file.RegistrationEnabled
	}
	if file.GuestCheckoutEnabled != nil {
		config.GuestCheckoutEnabled = *file.GuestCheckoutEnabled
	}
	if file.BalancePaymentEnabled != nil {
		config.BalancePaymentEnabled = *file.BalancePaymentEnabled
	}
	if file.OnlinePaymentEnabled != nil {
		config.OnlinePaymentEnabled = *file.OnlinePaymentEnabled
	}
	if file.ShowStockCount != nil {
		config.ShowStockCount = *file.ShowStockCount
	}
	if file.MaxPurchaseQuantity > 0 && os.Getenv("MAX_PURCHASE_QUANTITY") == "" {
		config.MaxPurchaseQuantity = file.MaxPurchaseQuantity
	}
	if file.PaymentTimeoutMinutes > 0 && os.Getenv("PAYMENT_TIMEOUT_MINUTES") == "" {
		config.PaymentTimeout = time.Duration(file.PaymentTimeoutMinutes) * time.Minute
	}
	return config, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
