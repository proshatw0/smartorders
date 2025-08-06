package jwt

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// сроки жизни в минутах
const (
	REFEXP = 14 * 24 * time.Hour
	ACCEXP = 10 * time.Minute

	KEY = "6h6h25f43h6996h245f7386h23459f78fh92456378f2345h9c678"
)

// струтура заголовка
type Header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid,omitempty"`
}

// струтура полезной нагрузки
type Payload struct {
	Iss string   `json:"iss,omitempty"` // издатель токена
	Sub string   `json:"sub,omitempty"` // субъект, которому выдан токен
	Aud []string `json:"aud,omitempty"` // получатели, которым предназначается данный токен
	Exp int64    `json:"exp,omitempty"` // время, когда токен станет невалидным
	Nbf int64    `json:"nbf,omitempty"` // время, с которого токен считается действительным
	Iat int64    `json:"iat,omitempty"` // время, в которое выдан токен
	Jti string   `json:"jti,omitempty"` // уникальный идентификатор токена
	Did string   `json:"did,omitempty"` // уникальный идентификатор устройства для которого предоставляется токен
	Tus string   `json:"tus,omitempty"` // тип токена
}

// струтура токена
type Token struct {
	Header    Header  // заголовок
	Payload   Payload // полезная нагрузка
	Signature string  // подпись
}

type EncodeOptions struct {
	TTL       time.Duration
	NotBefore *time.Time
	Issuer    string
	Now       time.Time
}

type EncodeOption func(*EncodeOptions)

func WithTTL(ttl time.Duration) EncodeOption {
	return func(o *EncodeOptions) { o.TTL = ttl }
}
func WithNotBefore(t time.Time) EncodeOption {
	return func(o *EncodeOptions) { o.NotBefore = &t }
}
func WithIssuer(iss string) EncodeOption {
	return func(o *EncodeOptions) { o.Issuer = iss }
}
func WithNow(t time.Time) EncodeOption {
	return func(o *EncodeOptions) { o.Now = t }
}

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
	const leeway int64 = 60
	if pay.Nbf != 0 && now+leeway < pay.Nbf {
		return nil, errors.New("token not active yet")
	}
	if pay.Exp != 0 && now-leeway > pay.Exp {
		return nil, errors.New("token expired")
	}

	return &Token{Header: hdr, Payload: pay, Signature: parts[2]}, nil
}
