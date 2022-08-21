package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/crunchy-labs/crunchyroll-go/v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SaveSession(crunchy *crunchyroll.Crunchyroll) error {
	file := filepath.Join(os.TempDir(), ".crunchy")
	return os.WriteFile(file, []byte(crunchy.RefreshToken), 0600)
}

func SaveCredentialsPersistent(user, password string, encryptionKey []byte) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	file := filepath.Join(configDir, "crunchy-cli", "crunchy")

	var credentials []byte
	if encryptionKey != nil {
		hashedEncryptionKey := sha256.Sum256(encryptionKey)
		block, err := aes.NewCipher(hashedEncryptionKey[:])
		if err != nil {
			return err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return err
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
			return err
		}
		b := gcm.Seal(nonce, nonce, []byte(fmt.Sprintf("%s\n%s", user, password)), nil)
		credentials = append([]byte("aes:"), b...)
	} else {
		credentials = []byte(fmt.Sprintf("%s\n%s", user, password))
	}

	if err = os.MkdirAll(filepath.Join(configDir, "crunchy-cli"), 0755); err != nil {
		return err
	}
	return os.WriteFile(file, credentials, 0600)
}

func SaveSessionPersistent(crunchy *crunchyroll.Crunchyroll) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	file := filepath.Join(configDir, "crunchy-cli", "crunchy")

	if err = os.MkdirAll(filepath.Join(configDir, "crunchy-cli"), 0755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(crunchy.RefreshToken), 0600)
}

func IsTempSession() bool {
	file := filepath.Join(os.TempDir(), ".crunchy")
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		return true
	}
	return false
}

func IsSavedSessionEncrypted() (bool, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return false, err
	}
	file := filepath.Join(configDir, "crunchy-cli", "crunchy")
	body, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(string(body), "aes:"), nil
}

func LoadSession(encryptionKey []byte) (*crunchyroll.Crunchyroll, error) {
	file := filepath.Join(os.TempDir(), ".crunchy")
	crunchy, err := loadTempSession(file)
	if err != nil {
		return nil, err
	}
	if crunchy != nil {
		return crunchy, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	file = filepath.Join(configDir, "crunchy-cli", "crunchy")
	crunchy, err = loadPersistentSession(file, encryptionKey)
	if err != nil {
		return nil, err
	}
	if crunchy != nil {
		return crunchy, nil
	}

	return nil, fmt.Errorf("not logged in")
}

func loadTempSession(file string) (*crunchyroll.Crunchyroll, error) {
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		crunchy, err := crunchyroll.LoginWithRefreshToken(string(body), SystemLocale(true), Client)
		if err != nil {
			Log.Debug("Failed to login with temp refresh token: %v", err)
		} else {
			Log.Debug("Logged in with refresh token %s. BLANK THIS LINE OUT IF YOU'RE ASKED TO POST THE DEBUG OUTPUT SOMEWHERE", body)
			return crunchy, nil
		}
	}
	return nil, nil
}

func loadPersistentSession(file string, encryptionKey []byte) (crunchy *crunchyroll.Crunchyroll, err error) {
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		split := strings.SplitN(string(body), "\n", 2)
		if len(split) == 1 || split[1] == "" && strings.HasPrefix(split[0], "aes:") {
			encrypted := body[4:]
			hashedEncryptionKey := sha256.Sum256(encryptionKey)
			block, err := aes.NewCipher(hashedEncryptionKey[:])
			if err != nil {
				return nil, err
			}
			gcm, err := cipher.NewGCM(block)
			if err != nil {
				return nil, err
			}
			nonce, cypherText := encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():]
			b, err := gcm.Open(nil, nonce, cypherText, nil)
			if err != nil {
				return nil, err
			}
			split = strings.SplitN(string(b), "\n", 2)
		}
		if len(split) == 2 {
			if crunchy, err = crunchyroll.LoginWithCredentials(split[0], split[1], SystemLocale(true), Client); err != nil {
				return nil, err
			}
			Log.Debug("Logged in with credentials")
		} else {
			if crunchy, err = crunchyroll.LoginWithRefreshToken(split[0], SystemLocale(true), Client); err != nil {
				return nil, err
			}
			Log.Debug("Logged in with refresh token %s. BLANK THIS LINE OUT IF YOU'RE ASKED TO POST THE DEBUG OUTPUT SOMEWHERE", split[0])
		}

		// the refresh token is written to a temp file to reduce the amount of re-logging in.
		// it seems like that crunchyroll has also a little cooldown if a user logs in too often in a short time
		if err = os.WriteFile(filepath.Join(os.TempDir(), ".crunchy"), []byte(crunchy.RefreshToken), 0600); err != nil {
			return nil, err
		}
	}

	return
}
