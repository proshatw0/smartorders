package jwt

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

func FromJSON(data []byte) (*Token, error) {
	type envelope struct {
		Header  *json.RawMessage `json:"header"`
		Payload *json.RawMessage `json:"payload"`
		Alg     *string          `json:"alg"`
		Kid     *string          `json:"kid"`
	}

	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}

	var hdr Header
	var pay Payload

	switch {
	case env.Header != nil || env.Payload != nil:
		if env.Header != nil {
			if err := json.Unmarshal(*env.Header, &hdr); err != nil {
				return nil, err
			}
		}
		if env.Payload != nil {
			if err := unmarshalPayloadFlexible(*env.Payload, &pay); err != nil {
				return nil, err
			}
		}
	default:
		if env.Alg != nil {
			hdr.Alg = *env.Alg
		}
		if env.Kid != nil {
			hdr.Kid = *env.Kid
		}
		if err := unmarshalPayloadFlexible(data, &pay); err != nil {
			return nil, err
		}
	}

	return &Token{Header: hdr, Payload: pay}, nil
}

func unmarshalPayloadFlexible(data []byte, out *Payload) error {
	type alias Payload
	var a alias
	_ = json.Unmarshal(data, &a)

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		if a.Sub == "" && len(a.Aud) == 0 && a.Jti == "" && a.Did == "" {
			return err
		}
	}

	if v, ok := m["iss"]; ok {
		_ = json.Unmarshal(v, &a.Iss)
	}
	if v, ok := m["sub"]; ok {
		_ = json.Unmarshal(v, &a.Sub)
	}
	if v, ok := m["jti"]; ok {
		_ = json.Unmarshal(v, &a.Jti)
	}
	if v, ok := m["did"]; ok {
		_ = json.Unmarshal(v, &a.Did)
	}
	if v, ok := m["token_use"]; ok {
		_ = json.Unmarshal(v, &a.Tus)
	}

	if v, ok := m["aud"]; ok {
		slice, err := parseStringOrStringSlice(v)
		if err != nil {
			return err
		}
		a.Aud = slice
	}

	if v, ok := m["exp"]; ok {
		if ts, err := parseFlexibleTime(v); err == nil {
			a.Exp = ts
		}
	}
	if v, ok := m["nbf"]; ok {
		if ts, err := parseFlexibleTime(v); err == nil {
			a.Nbf = ts
		}
	}
	if v, ok := m["iat"]; ok {
		if ts, err := parseFlexibleTime(v); err == nil {
			a.Iat = ts
		}
	}

	*out = Payload(a)
	return nil
}

func parseStringOrStringSlice(raw json.RawMessage) ([]string, error) {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var anyArr []any
	if err := json.Unmarshal(raw, &anyArr); err == nil {
		out := make([]string, 0, len(anyArr))
		for _, it := range anyArr {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, errors.New("aud must be string or array of strings")
}

func parseFlexibleTime(raw json.RawMessage) (int64, error) {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if sec, err2 := parseInt64String(s); err2 == nil {
			return sec, nil
		}
		if tm, err3 := time.Parse(time.RFC3339, s); err3 == nil {
			return tm.Unix(), nil
		}
	}
	return 0, errors.New("invalid time format")
}

func parseInt64String(s string) (int64, error) {
	s = strings.TrimSpace(s)
	return strconvParseInt10(s)
}

func strconvParseInt10(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (t *Token) ToJSONEnvelope() ([]byte, error) {
	type env struct {
		Header  Header  `json:"header"`
		Payload Payload `json:"payload"`
	}
	return json.Marshal(env{Header: t.Header, Payload: t.Payload})
}

func (t *Token) ToJSONEnvelopeIndent() ([]byte, error) {
	type env struct {
		Header  Header  `json:"header"`
		Payload Payload `json:"payload"`
	}
	return json.MarshalIndent(env{Header: t.Header, Payload: t.Payload}, "", "  ")
}

func (t *Token) ToJSONFlat() ([]byte, error) {
	m := make(map[string]any)

	if t.Header.Alg != "" {
		m["alg"] = t.Header.Alg
	}
	if t.Header.Kid != "" {
		m["kid"] = t.Header.Kid
	}

	pb, err := json.Marshal(t.Payload)
	if err != nil {
		return nil, err
	}
	var pm map[string]any
	if err := json.Unmarshal(pb, &pm); err != nil {
		return nil, err
	}
	for k, v := range pm {
		m[k] = v
	}

	return json.Marshal(m)
}

func (t *Token) ToJSONFlatIndent() ([]byte, error) {
	m := make(map[string]any)

	if t.Header.Alg != "" {
		m["alg"] = t.Header.Alg
	}
	if t.Header.Kid != "" {
		m["kid"] = t.Header.Kid
	}

	pb, err := json.Marshal(t.Payload)
	if err != nil {
		return nil, err
	}
	var pm map[string]any
	if err := json.Unmarshal(pb, &pm); err != nil {
		return nil, err
	}
	for k, v := range pm {
		m[k] = v
	}

	return json.MarshalIndent(m, "", "  ")
}
