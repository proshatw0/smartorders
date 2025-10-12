package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// REFEXP — срок жизни refresh-токена (14 суток).
// ACCEXP — срок жизни access-токена (10 минут).
const (
	REFEXP = 14 * 24 * time.Hour
	ACCEXP = 10 * time.Minute
)

// Header — структура заголовка JWT.
// Поле Alg фиксировано ("EdDSA"), Kid — идентификатор ключа.
type Header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid,omitempty"`
}

// Payload — структура полезной нагрузки JWT.
// Содержит стандартные поля RFC 7519 и дополнительные (`Did`, `Tus`).
type Payload struct {
	Iss string   `json:"iss,omitempty"` // Issuer — издатель токена
	Sub string   `json:"sub,omitempty"` // Subject — субъект (пользователь)
	Aud []string `json:"aud,omitempty"` // Audience — получатели токена
	Exp int64    `json:"exp,omitempty"` // Expiration — момент истечения
	Nbf int64    `json:"nbf,omitempty"` // Not Before — начало действия
	Iat int64    `json:"iat,omitempty"` // Issued At — время выдачи
	Jti string   `json:"jti,omitempty"` // JWT ID — уникальный идентификатор
	Did string   `json:"did,omitempty"` // Device ID — уникальное устройство
	Tus string   `json:"tus,omitempty"` // Token Use — тип токена ("access"/"refresh")
}

// Token — полная структура JWT.
// Объединяет заголовок, полезную нагрузку и подпись.
type Token struct {
	Header    Header
	Payload   Payload
	Signature string
}

// EncodeOptions — параметры для генерации токена.
// Используются через функциональные опции.
type EncodeOptions struct {
	TTL       time.Duration // срок жизни токена
	NotBefore *time.Time    // момент начала действия
	Issuer    string        // издатель
	Now       time.Time     // текущее время (для тестов или переопределения)
}

// EncodeOption — функция, изменяющая EncodeOptions.
type EncodeOption func(*EncodeOptions)

// WithTTL — задаёт срок жизни токена.
func WithTTL(ttl time.Duration) EncodeOption {
	return func(o *EncodeOptions) { o.TTL = ttl }
}

// WithNotBefore — задаёт момент, с которого токен становится активным.
func WithNotBefore(t time.Time) EncodeOption {
	return func(o *EncodeOptions) { o.NotBefore = &t }
}

// WithIssuer — задаёт издателя токена.
func WithIssuer(iss string) EncodeOption {
	return func(o *EncodeOptions) { o.Issuer = iss }
}

// WithNow — задаёт текущее время (используется для тестов).
func WithNow(t time.Time) EncodeOption {
	return func(o *EncodeOptions) { o.Now = t }
}

// Encode — создаёт и подписывает JWT с использованием Ed25519.
// Возвращает строковое представление токена в формате base64(header).base64(payload).base64(signature).
func (t *Token) Encode(priv ed25519.PrivateKey, opts ...EncodeOption) (string, error) {
	if t.Payload.Sub == "" {
		return "", errors.New("sub is empty")
	}
	if len(t.Payload.Aud) == 0 {
		return "", errors.New("aud is empty")
	}
	if t.Payload.Jti == "" {
		return "", errors.New("jti is empty")
	}
	if t.Payload.Did == "" {
		return "", errors.New("did is empty")
	}
	if t.Payload.Tus == "" {
		t.Payload.Tus = "access"
	}

	o := &EncodeOptions{}
	for _, f := range opts {
		f(o)
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}

	t.Header.Alg = "EdDSA"

	if t.Payload.Iss == "" {
		if o.Issuer != "" {
			t.Payload.Iss = o.Issuer
		} else {
			t.Payload.Iss = "user-service"
		}
	}

	if t.Payload.Iat == 0 {
		t.Payload.Iat = now.Unix()
	}
	if t.Payload.Nbf == 0 {
		if o.NotBefore != nil {
			t.Payload.Nbf = o.NotBefore.Unix()
		} else {
			t.Payload.Nbf = t.Payload.Iat
		}
	}
	ttl := o.TTL
	if ttl == 0 {
		switch strings.ToLower(t.Payload.Tus) {
		case "access":
			ttl = ACCEXP
		case "refresh":
			ttl = REFEXP
		default:
			ttl = ACCEXP
		}
	}
	if t.Payload.Exp == 0 {
		t.Payload.Exp = now.Add(ttl).Unix()
	}
	hb, err := json.Marshal(t.Header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(t.Payload)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(hb)
	p := base64.RawURLEncoding.EncodeToString(pb)
	signing := h + "." + p

	sig := ed25519.Sign(priv, []byte(signing))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	t.Signature = sigB64
	return signing + "." + sigB64, nil
}

// Verify — проверяет подпись и валидность JWT.
// Проверяет структуру токена, алгоритм, подпись и временные ограничения.
func Verify(token string, pub ed25519.PublicKey) (*Token, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}

	var hdr Header
	if err := json.Unmarshal(hb, &hdr); err != nil {
		return nil, err
	}
	if hdr.Alg != "EdDSA" {
		return nil, errors.New("unexpected alg: " + hdr.Alg)
	}

	signing := parts[0] + "." + parts[1]
	if !ed25519.Verify(pub, []byte(signing), sig) {
		return nil, errors.New("signature mismatch")
	}

	var pay Payload
	if err := json.Unmarshal(pb, &pay); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	const leeway int64 = 30 // допуск 30 секунд
	if pay.Nbf != 0 && now+leeway < pay.Nbf {
		return nil, errors.New("token not active yet")
	}
	if pay.Exp != 0 && now-leeway > pay.Exp {
		return nil, errors.New("token expired")
	}

	return &Token{Header: hdr, Payload: pay, Signature: parts[2]}, nil
}
