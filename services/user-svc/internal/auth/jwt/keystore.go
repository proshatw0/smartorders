package jwt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type KeySet struct {
	Name      string `json:"name"`
	Kid       string `json:"kid"`
	Alg       string `json:"alg"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"created_at"`

	PublicB64  string `json:"public_key"`
	PrivateB64 string `json:"private_key"`
}

type keystorePlain struct {
	Version int      `json:"version"`
	Sets    []KeySet `json:"key_sets"`
}

type keystoreEncrypted struct {
	Version   int    `json:"version"`
	NonceB64  string `json:"nonce"`
	CipherB64 string `json:"ciphertext"`
}

func EnsureKeysDir() (string, error) {
	if p := os.Getenv("SMARTORDERS_KEYSTORE_PATH"); p != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		return p, nil
	}
	baseDir := os.Getenv("SMARTORDERS_KEYS_DIR")
	if baseDir == "" {
		d, err := defaultBaseDir()
		if err != nil {
			return "", err
		}
		baseDir = filepath.Join(d, "keys")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "keystore.enc"), nil
}

func LoadKeystore() (*keystorePlain, string, error) {
	path, err := EnsureKeysDir()
	if err != nil {
		return nil, "", err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &keystorePlain{Version: 1, Sets: []KeySet{}}, path, nil
		}
		return nil, "", err
	}

	mk, err := masterKey()
	if err != nil {
		return nil, "", err
	}

	var enc keystoreEncrypted
	if err := json.Unmarshal(b, &enc); err != nil {
		return nil, "", fmt.Errorf("keystore json: %w", err)
	}
	if enc.Version != 1 {
		return nil, "", fmt.Errorf("unsupported keystore version: %d", enc.Version)
	}

	nonce, err := base64.RawStdEncoding.DecodeString(enc.NonceB64)
	if err != nil {
		return nil, "", err
	}
	ct, err := base64.RawStdEncoding.DecodeString(enc.CipherB64)
	if err != nil {
		return nil, "", err
	}

	pt, err := aesGCMDecrypt(mk, nonce, ct)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt keystore: %w", err)
	}

	var plain keystorePlain
	if err := json.Unmarshal(pt, &plain); err != nil {
		return nil, "", err
	}
	if plain.Version == 0 {
		plain.Version = 1
	}
	return &plain, path, nil
}

func SaveKeystore(store *keystorePlain, path string) error {
	mk, err := masterKey()
	if err != nil {
		return err
	}
	store.Version = 1
	pt, err := json.Marshal(store)
	if err != nil {
		return err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	ct, err := aesGCMEncrypt(mk, nonce, pt)
	if err != nil {
		return err
	}
	enc := keystoreEncrypted{
		Version:   1,
		NonceB64:  base64.RawStdEncoding.EncodeToString(nonce),
		CipherB64: base64.RawStdEncoding.EncodeToString(ct),
	}
	out, err := json.MarshalIndent(enc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

func CreateKeySet(name string) (string, string, error) {
	if strings.TrimSpace(name) == "" {
		return "", "", errors.New("name is empty")
	}
	store, path, err := LoadKeystore()
	if err != nil {
		return "", "", err
	}
	// уникальность имени
	for _, s := range store.Sets {
		if strings.EqualFold(s.Name, name) {
			return "", "", fmt.Errorf("key set with name %q already exists", name)
		}
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	kid := fmt.Sprintf("ed-%s-%d", sanitizeNameForKid(name), time.Now().Unix())

	ks := KeySet{
		Name:       name,
		Kid:        kid,
		Alg:        "EdDSA",
		Active:     false, // по умолчанию выключен
		CreatedAt:  time.Now().Unix(),
		PublicB64:  base64.RawStdEncoding.EncodeToString(pub),
		PrivateB64: base64.RawStdEncoding.EncodeToString(priv),
	}
	store.Sets = append(store.Sets, ks)

	if err := SaveKeystore(store, path); err != nil {
		return "", "", err
	}
	return name, kid, nil
}

func ActivateKeySet(nameOrKid string, exclusive bool) error {
	store, path, err := LoadKeystore()
	if err != nil {
		return err
	}
	found := false
	for i := range store.Sets {
		s := &store.Sets[i]
		match := strings.EqualFold(s.Name, nameOrKid) || strings.EqualFold(s.Kid, nameOrKid)
		if match {
			s.Active = true
			found = true
		} else if exclusive {
			s.Active = false
		}
	}
	if !found {
		return fmt.Errorf("key set %q not found", nameOrKid)
	}
	return SaveKeystore(store, path)
}

func ListKeySets() ([]KeySet, error) {
	store, _, err := LoadKeystore()
	if err != nil {
		return nil, err
	}
	return append([]KeySet(nil), store.Sets...), nil
}

func GetActiveSigningKey() (ed25519.PrivateKey, string, error) {
	store, _, err := LoadKeystore()
	if err != nil {
		return nil, "", err
	}
	for _, s := range store.Sets {
		if s.Active && s.Alg == "EdDSA" {
			privBytes, err := base64.RawStdEncoding.DecodeString(s.PrivateB64)
			if err != nil {
				return nil, "", err
			}
			return ed25519.PrivateKey(privBytes), s.Kid, nil
		}
	}
	return nil, "", errors.New("no active key set")
}

func GetPublicKeyByKid(kid string) (ed25519.PublicKey, error) {
	store, _, err := LoadKeystore()
	if err != nil {
		return nil, err
	}
	for _, s := range store.Sets {
		if s.Kid == kid && s.Alg == "EdDSA" {
			pubBytes, err := base64.RawStdEncoding.DecodeString(s.PublicB64)
			if err != nil {
				return nil, err
			}
			return ed25519.PublicKey(pubBytes), nil
		}
	}
	return nil, fmt.Errorf("public key for kid %q not found", kid)
}

func ExportJWKS(onlyActive bool) ([]byte, error) {
	store, _, err := LoadKeystore()
	if err != nil {
		return nil, err
	}
	type jwk struct {
		Kty string `json:"kty"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Alg string `json:"alg"`
		Use string `json:"use"`
		Kid string `json:"kid"`
	}
	out := struct {
		Keys []jwk `json:"keys"`
	}{Keys: []jwk{}}

	for _, s := range store.Sets {
		if s.Alg != "EdDSA" {
			continue
		}
		if onlyActive && !s.Active {
			continue
		}
		out.Keys = append(out.Keys, jwk{
			Kty: "OKP",
			Crv: "Ed25519",
			X:   s.PublicB64,
			Alg: "EdDSA",
			Use: "sig",
			Kid: s.Kid,
		})
	}
	return json.MarshalIndent(out, "", "  ")
}

func defaultBaseDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("AppData")
		if appdata == "" {
			return "", errors.New("AppData is empty")
		}
		return filepath.Join(appdata, "SmartOrders"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "SmartOrders"), nil
	default:
		dir := os.Getenv("XDG_CONFIG_HOME")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, ".config")
		}
		return filepath.Join(dir, "smartorders"), nil
	}
}

func sanitizeNameForKid(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	return s
}

func masterKey() ([]byte, error) {
	raw := os.Getenv("SMARTORDERS_KEYSTORE_MASTER_KEY")
	if raw == "" {
		return nil, errors.New("SMARTORDERS_KEYSTORE_MASTER_KEY is not set (need 32 bytes or base64)")
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && len(b) == 32 {
		return b, nil
	}
	b2, err2 := base64.RawStdEncoding.DecodeString(raw)
	if err2 == nil && len(b2) == 32 {
		return b2, nil
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

func aesGCMEncrypt(key, nonce, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("aes key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("bad nonce size: %d", len(nonce))
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("aes key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("bad nonce size: %d", len(nonce))
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
