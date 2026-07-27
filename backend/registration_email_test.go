package main

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestRegistrationEmailMessageContainsTextAndHTMLAlternatives(t *testing.T) {
	raw := registrationEmailMessage(
		&mail.Address{Address: "sender@example.test"},
		&mail.Address{Address: "123456@qq.com"},
		"482913",
		10*time.Minute,
	)
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	sender, err := mail.ParseAddress(message.Header.Get("From"))
	if err != nil {
		t.Fatal(err)
	}
	if sender.Name != "阿巳" || sender.Address != "sender@example.test" {
		t.Fatalf("unexpected sender: %+v", sender)
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "multipart/alternative" || params["boundary"] == "" {
		t.Fatalf("unexpected content type: %q", message.Header.Get("Content-Type"))
	}

	parts := map[string]string{}
	reader := multipart.NewReader(message.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		partType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatal(err)
		}
		parts[partType] = string(body)
	}

	if !strings.Contains(parts["text/plain"], "482913") || !strings.Contains(parts["text/plain"], "10 分钟") {
		t.Fatalf("plain text fallback is incomplete: %q", parts["text/plain"])
	}
	htmlBody := parts["text/html"]
	for _, expected := range []string{"完成你的阿巳注册", "482913", "10 分钟", `role="presentation"`} {
		if !strings.Contains(htmlBody, expected) {
			t.Fatalf("HTML body is missing %q", expected)
		}
	}
}
