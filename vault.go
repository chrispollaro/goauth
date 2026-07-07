package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

type Entry struct {
	Issuer  string `json:"issuer"`
	Account string `json:"account"`
	Secret  string `json:"secret"` // base32
	Digits  int    `json:"digits"`
	Period  int    `json:"period"`
}

type Vault struct {
	Entries []Entry `json:"entries"`
}

type vaultFile struct {
	Salt  []byte `json:"salt"`
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}

func SaveVault(path, password string, v *Vault) error {
	plain, err := json.Marshal(v)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	block, err := aes.NewCipher(deriveKey(password, salt))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	vf := vaultFile{Salt: salt, Nonce: nonce, Data: gcm.Seal(nil, nonce, plain, nil)}
	out, err := json.Marshal(vf)
	if err != nil {
		return err
	}
	// write atomically
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadVault(path, password string) (*Vault, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Vault{}, nil // fresh vault
	}
	if err != nil {
		return nil, err
	}
	var vf vaultFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(deriveKey(password, vf.Salt))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, vf.Nonce, vf.Data, nil)
	if err != nil {
		return nil, errors.New("wrong password or corrupted vault")
	}
	var v Vault
	if err := json.Unmarshal(plain, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func vaultPath(portable bool) string {
	if portable {
		exe, err := os.Executable()
		if err == nil {
			return filepath.Join(filepath.Dir(exe), "vault.enc")
		}
	}
	dir, _ := os.UserConfigDir()
	dir = filepath.Join(dir, "goauth")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "vault.enc")
}
