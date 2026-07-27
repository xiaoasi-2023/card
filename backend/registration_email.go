package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const registrationVerificationPurpose = "registration"

var (
	errVerificationCodeInvalid = errors.New("verification code is invalid")
	errVerificationCodeExpired = errors.New("verification code has expired")
)

type registrationMailer interface {
	SendRegistrationCode(context.Context, string, string, time.Duration) error
}

type smtpRegistrationMailer struct {
	host     string
	port     int
	username string
	password string
	from     string
	tls      bool
}

func newSMTPMailer(config Config) registrationMailer {
	return &smtpRegistrationMailer{
		host:     config.SMTPHost,
		port:     config.SMTPPort,
		username: config.SMTPUsername,
		password: config.SMTPPassword,
		from:     config.SMTPFrom,
		tls:      config.SMTPTLS,
	}
}

func (m *smtpRegistrationMailer) SendRegistrationCode(ctx context.Context, recipient, code string, ttl time.Duration) error {
	from, err := mail.ParseAddress(m.from)
	if err != nil {
		return errors.New("invalid SMTP sender")
	}
	to, err := mail.ParseAddress(recipient)
	if err != nil || !strings.EqualFold(to.Address, recipient) {
		return errors.New("invalid mail recipient")
	}

	address := net.JoinHostPort(m.host, strconv.Itoa(m.port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var connection net.Conn
	if m.tls {
		tlsDialer := &tls.Dialer{
			NetDialer: dialer,
			Config:    &tls.Config{MinVersion: tls.VersionTLS12, ServerName: m.host},
		}
		connection, err = tlsDialer.DialContext(ctx, "tcp", address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	}

	client, err := smtp.NewClient(connection, m.host)
	if err != nil {
		return fmt.Errorf("initialize SMTP client: %w", err)
	}
	defer client.Close()
	if m.username != "" {
		if err := client.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			return fmt.Errorf("authenticate SMTP client: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	if _, err := writer.Write(registrationEmailMessage(from, to, code, ttl)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP client: %w", err)
	}
	return nil
}

func registrationEmailMessage(from, to *mail.Address, code string, ttl time.Duration) []byte {
	var message bytes.Buffer
	displayFrom := *from
	if displayFrom.Name == "" {
		displayFrom.Name = "阿巳"
	}
	subject := mime.QEncoding.Encode("UTF-8", "阿巳注册验证码")
	fmt.Fprintf(&message, "From: %s\r\n", displayFrom.String())
	fmt.Fprintf(&message, "To: %s\r\n", to.String())
	fmt.Fprintf(&message, "Subject: %s\r\n", subject)
	message.WriteString("MIME-Version: 1.0\r\n")
	multipartWriter := multipart.NewWriter(&message)
	contentType := mime.FormatMediaType("multipart/alternative", map[string]string{
		"boundary": multipartWriter.Boundary(),
	})
	fmt.Fprintf(&message, "Content-Type: %s\r\n\r\n", contentType)

	minutes := max(1, int(ttl.Minutes()))
	textBody := fmt.Sprintf(
		"欢迎注册阿巳。\n\n您的注册验证码是：%s\n\n验证码将在 %d 分钟后失效，请勿向任何人泄露。若非本人操作，请忽略此邮件。",
		code,
		minutes,
	)
	writeRegistrationEmailPart(multipartWriter, "text/plain; charset=UTF-8", textBody)
	writeRegistrationEmailPart(
		multipartWriter,
		"text/html; charset=UTF-8",
		registrationEmailHTML(code, minutes),
	)
	_ = multipartWriter.Close()
	return message.Bytes()
}

func writeRegistrationEmailPart(writer *multipart.Writer, contentType, body string) {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(header)
	if err != nil {
		return
	}
	encoder := quotedprintable.NewWriter(part)
	_, _ = encoder.Write([]byte(body))
	_ = encoder.Close()
}

func registrationEmailHTML(code string, minutes int) string {
	const beforeCode = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>阿巳注册验证码</title>
</head>
<body style="margin:0;padding:0;background:#f2f4f7;color:#17202a;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',Arial,sans-serif;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="width:100%;background:#f2f4f7;">
    <tr>
      <td align="center" style="padding:40px 16px;">
        <table role="presentation" width="600" cellspacing="0" cellpadding="0" border="0" style="width:100%;max-width:600px;background:#ffffff;border:1px solid #e1e6eb;border-radius:8px;overflow:hidden;box-shadow:0 14px 40px rgba(17,24,39,0.08);">
          <tr>
            <td style="padding:24px 32px;background:#070a0d;">
              <table role="presentation" cellspacing="0" cellpadding="0" border="0">
                <tr>
                  <td style="width:38px;height:38px;text-align:center;vertical-align:middle;border-radius:5px;background:#1677ff;color:#ffffff;font-size:22px;font-weight:800;">巳</td>
                  <td style="padding-left:12px;color:#ffffff;font-size:21px;font-weight:800;">阿巳</td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:42px 40px 36px;">
              <p style="margin:0 0 10px;color:#1677ff;font-size:13px;font-weight:700;letter-spacing:0;">账号安全验证</p>
              <h1 style="margin:0 0 14px;color:#111827;font-size:28px;line-height:1.35;font-weight:800;letter-spacing:0;">完成你的阿巳注册</h1>
              <p style="margin:0 0 28px;color:#596574;font-size:15px;line-height:1.8;">你正在注册阿巳账号，请在注册页面输入以下验证码：</p>
              <div style="padding:22px 18px;border:1px solid #dbe4ee;border-radius:7px;background:#f6f9fc;text-align:center;">
                <p style="margin:0 0 8px;color:#7b8794;font-size:12px;font-weight:600;">注册验证码</p>
                <p style="margin:0;color:#0969da;font-family:Consolas,'SFMono-Regular','Courier New',monospace;font-size:36px;line-height:1.2;font-weight:800;letter-spacing:8px;">`
	const afterCodeBeforeMinutes = `</p>
              </div>
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="width:100%;margin-top:24px;">
                <tr>
                  <td style="padding:14px 16px;border-left:3px solid #1677ff;background:#f7f9fc;color:#4b5563;font-size:13px;line-height:1.7;">
                    验证码将在 <strong style="color:#111827;">`
	const afterMinutes = ` 分钟</strong>后失效，请勿转发或告知他人。
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;color:#7b8794;font-size:13px;line-height:1.75;">若这不是你的操作，请忽略此邮件。你的账号不会因此受到影响。</p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 32px;border-top:1px solid #edf0f3;background:#fafbfc;color:#929ba6;font-size:12px;line-height:1.7;text-align:center;">
              此邮件由阿巳系统自动发送，请勿直接回复。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`
	return beforeCode + html.EscapeString(code) + afterCodeBeforeMinutes + strconv.Itoa(minutes) + afterMinutes
}

func (a *App) sendRegistrationCode(c *gin.Context) {
	if !a.config.RegistrationEnabled {
		fail(c, http.StatusForbidden, "registration_disabled", "当前未开放注册")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !bindJSON(c, &req) {
		return
	}
	email, ok := normalizeQQEmail(req.Email)
	if !ok {
		fail(c, http.StatusBadRequest, "invalid_email", "仅支持 QQ 邮箱注册")
		return
	}
	if !a.registrationEmailConfigured() {
		fail(c, http.StatusServiceUnavailable, "email_service_unavailable", "邮件服务暂不可用")
		return
	}

	a.verificationMu.Lock()
	defer a.verificationMu.Unlock()

	var existing User
	if err := a.db.Where("LOWER(email) = ?", email).First(&existing).Error; err == nil {
		fail(c, http.StatusConflict, "account_exists", "邮箱已注册")
		return
	} else if !isNotFound(err) {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}

	now := time.Now()
	var previous EmailVerification
	err := a.db.Where("email = ? AND purpose = ? AND consumed_at IS NULL", email, registrationVerificationPurpose).
		Order("id DESC").First(&previous).Error
	if err == nil {
		retryAt := previous.SentAt.Add(a.config.RegistrationCodeResendInterval)
		if retryAt.After(now) {
			retryAfter := int(time.Until(retryAt).Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			fail(c, http.StatusTooManyRequests, "verification_code_throttled", "验证码发送过于频繁")
			return
		}
	} else if !isNotFound(err) {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}

	code, err := generateVerificationCode()
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}
	if err := a.mailer.SendRegistrationCode(c.Request.Context(), email, code, a.config.RegistrationCodeTTL); err != nil {
		fail(c, http.StatusBadGateway, "email_delivery_failed", "验证码发送失败，请稍后重试")
		return
	}

	verification := EmailVerification{
		Email:     email,
		Purpose:   registrationVerificationPurpose,
		CodeHash:  a.registrationCodeHash(email, code),
		ExpiresAt: now.Add(a.config.RegistrationCodeTTL),
		SentAt:    now,
	}
	err = a.db.Transaction(func(tx *gorm.DB) error {
		consumedAt := now
		if err := tx.Model(&EmailVerification{}).
			Where("email = ? AND purpose = ? AND consumed_at IS NULL", email, registrationVerificationPurpose).
			Update("consumed_at", consumedAt).Error; err != nil {
			return err
		}
		return tx.Create(&verification).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"expires_in_seconds": int(a.config.RegistrationCodeTTL.Seconds())})
}

func (a *App) register(c *gin.Context) {
	if !a.config.RegistrationEnabled {
		fail(c, http.StatusForbidden, "registration_disabled", "当前未开放注册")
		return
	}
	var req struct {
		Username         string `json:"username"`
		Email            string `json:"email"`
		Password         string `json:"password"`
		VerificationCode string `json:"verification_code"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	email, emailOK := normalizeQQEmail(req.Email)
	code := strings.TrimSpace(req.VerificationCode)
	if len(req.Username) < 3 || len(req.Username) > 64 || !emailOK || len(req.Password) < 8 || len(req.Password) > 72 || !validVerificationCode(code) {
		fail(c, http.StatusBadRequest, "invalid_account", "账号信息或验证码不符合要求")
		return
	}
	password, err := passwordHash(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}

	a.verificationMu.Lock()
	defer a.verificationMu.Unlock()

	now := time.Now()
	user := User{Username: req.Username, Email: email, PasswordHash: password, Role: "user", Status: "active"}
	var verificationFailure error
	err = a.db.Transaction(func(tx *gorm.DB) error {
		var verification EmailVerification
		err := tx.Where("email = ? AND purpose = ? AND consumed_at IS NULL", email, registrationVerificationPurpose).
			Order("id DESC").First(&verification).Error
		if isNotFound(err) {
			verificationFailure = errVerificationCodeInvalid
			return nil
		}
		if err != nil {
			return err
		}
		if !verification.ExpiresAt.After(now) {
			verificationFailure = errVerificationCodeExpired
			return nil
		}
		if verification.Attempts >= a.config.RegistrationCodeMaxAttempts ||
			!hmac.Equal([]byte(verification.CodeHash), []byte(a.registrationCodeHash(email, code))) {
			verification.Attempts++
			updates := map[string]any{"attempts": verification.Attempts}
			if verification.Attempts >= a.config.RegistrationCodeMaxAttempts {
				updates["consumed_at"] = now
			}
			if err := tx.Model(&EmailVerification{}).Where("id = ?", verification.ID).Updates(updates).Error; err != nil {
				return err
			}
			verificationFailure = errVerificationCodeInvalid
			return nil
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		update := tx.Model(&EmailVerification{}).
			Where("id = ? AND consumed_at IS NULL", verification.ID).
			Update("consumed_at", now)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errVerificationCodeInvalid
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			fail(c, http.StatusConflict, "account_exists", "用户名或邮箱已存在")
			return
		}
		if errors.Is(err, errVerificationCodeInvalid) {
			fail(c, http.StatusBadRequest, "invalid_verification_code", "验证码无效")
			return
		}
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}
	if errors.Is(verificationFailure, errVerificationCodeExpired) {
		fail(c, http.StatusBadRequest, "verification_code_expired", "验证码已过期")
		return
	}
	if verificationFailure != nil {
		fail(c, http.StatusBadRequest, "invalid_verification_code", "验证码无效")
		return
	}
	token, err := issueToken(a.config.JWTSecret, user)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal_error", "系统繁忙，请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "user": user})
}

func (a *App) registrationEmailConfigured() bool {
	return a.mailer != nil &&
		strings.TrimSpace(a.config.SMTPHost) != "" &&
		a.config.SMTPPort > 0 &&
		strings.TrimSpace(a.config.SMTPUsername) != "" &&
		a.config.SMTPPassword != "" &&
		strings.TrimSpace(a.config.SMTPFrom) != "" &&
		a.config.RegistrationCodeHashKey != "" &&
		a.config.RegistrationCodeTTL > 0 &&
		a.config.RegistrationCodeResendInterval > 0 &&
		a.config.RegistrationCodeMaxAttempts > 0
}

func (a *App) registrationCodeHash(email, code string) string {
	return keyedHash(a.config.RegistrationCodeHashKey, email+"|"+registrationVerificationPurpose+"|"+code)
}

func generateVerificationCode() (string, error) {
	number, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", number.Int64()), nil
}

func validVerificationCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func normalizeQQEmail(value string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(value))
	if len(email) > 254 || strings.Count(email, "@") != 1 {
		return "", false
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts[0]) == 0 || len(parts[0]) > 64 || parts[1] != "qq.com" {
		return "", false
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", false
	}
	return email, true
}
