package commands

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/ByteDream/crunchyroll-go/v3"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
)

var (
	loginPersistentFlag bool
	loginEncryptFlag    bool

	loginSessionIDFlag bool
	loginEtpRtFlag     bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to crunchyroll",
	Args:  cobra.RangeArgs(1, 2),

	RunE: func(cmd *cobra.Command, args []string) error {
		if loginSessionIDFlag {
			return loginSessionID(args[0])
		} else if loginEtpRtFlag {
			return loginEtpRt(args[0])
		} else {
			return loginCredentials(args[0], args[1])
		}
	},
}

func init() {
	loginCmd.Flags().BoolVar(&loginPersistentFlag,
		"persistent",
		false,
		"If the given credential should be stored persistent")
	loginCmd.Flags().BoolVar(&loginEncryptFlag,
		"encrypt",
		false,
		"Encrypt the given credentials (won't do anything if --session-id is given or --persistent is not given)")

	loginCmd.Flags().BoolVar(&loginSessionIDFlag,
		"session-id",
		false,
		"Use a session id to login instead of username and password")
	loginCmd.Flags().BoolVar(&loginEtpRtFlag,
		"etp-rt",
		false,
		"Use a etp rt cookie to login instead of username and password")

	rootCmd.AddCommand(loginCmd)
}

func loginCredentials(user, password string) error {
	out.Debug("Logging in via credentials")
	c, err := crunchyroll.LoginWithCredentials(user, password, systemLocale(false), client)
	if err != nil {
		return err
	}

	if loginPersistentFlag {
		if configDir, err := os.UserConfigDir(); err != nil {
			return fmt.Errorf("could not save credentials persistent: %w", err)
		} else {
			var credentials []byte

			if loginEncryptFlag {
				var passwd []byte

				for {
					fmt.Print("Enter password: ")
					passwd, err = readLineSilent()
					if err != nil {
						return err
					}
					fmt.Println()

					fmt.Print("Enter password again: ")
					repasswd, err := readLineSilent()
					if err != nil {
						return err
					}
					fmt.Println()

					if !bytes.Equal(passwd, repasswd) {
						fmt.Println("Passwords does not match, try again")
						continue
					}

					hashedPassword := sha256.Sum256(passwd)
					block, err := aes.NewCipher(hashedPassword[:])
					if err != nil {
						out.Err("Failed to create block: %w", err)
						os.Exit(1)
					}
					gcm, err := cipher.NewGCM(block)
					if err != nil {
						out.Err("Failed to create gcm: %w", err)
						os.Exit(1)
					}
					nonce := make([]byte, gcm.NonceSize())
					if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
						out.Err("Failed to fill nonce: %w", err)
						os.Exit(1)
					}

					b := gcm.Seal(nonce, nonce, []byte(fmt.Sprintf("%s\n%s", user, password)), nil)
					credentials = append([]byte("aes:"), b...)

					break
				}
			} else {
				credentials = []byte(fmt.Sprintf("%s\n%s", user, password))
			}

			os.MkdirAll(filepath.Join(configDir, "crunchy-cli"), 0755)
			if err = os.WriteFile(filepath.Join(configDir, "crunchy-cli", "crunchy"), credentials, 0600); err != nil {
				return err
			}
			if !loginEncryptFlag {
				out.Info("The login information will be stored permanently UNENCRYPTED on your drive (%s). "+
					"To encrypt it, use the `--encrypt` flag", filepath.Join(configDir, "crunchy-cli", "crunchy"))
			}
		}
	}

	if err = os.WriteFile(filepath.Join(os.TempDir(), ".crunchy"), []byte(c.EtpRt), 0600); err != nil {
		return err
	}

	if !loginPersistentFlag {
		out.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}

func loginSessionID(sessionID string) error {
	out.Debug("Logging in via session id")
	var c *crunchyroll.Crunchyroll
	var err error
	if c, err = crunchyroll.LoginWithSessionID(sessionID, systemLocale(false), client); err != nil {
		out.Err(err.Error())
		os.Exit(1)
	}

	if loginPersistentFlag {
		if configDir, err := os.UserConfigDir(); err != nil {
			return fmt.Errorf("could not save credentials persistent: %w", err)
		} else {
			os.MkdirAll(filepath.Join(configDir, "crunchy-cli"), 0755)
			if err = os.WriteFile(filepath.Join(configDir, "crunchy-cli", "crunchy"), []byte(c.EtpRt), 0600); err != nil {
				return err
			}
			out.Info("The login information will be stored permanently UNENCRYPTED on your drive (%s)", filepath.Join(configDir, "crunchy-cli", "crunchy"))
		}
	}
	if err = os.WriteFile(filepath.Join(os.TempDir(), ".crunchy"), []byte(c.EtpRt), 0600); err != nil {
		return err
	}

	if !loginPersistentFlag {
		out.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}

func loginEtpRt(etpRt string) error {
	out.Debug("Logging in via etp rt")
	if _, err := crunchyroll.LoginWithEtpRt(etpRt, systemLocale(false), client); err != nil {
		out.Err(err.Error())
		os.Exit(1)
	}

	var err error
	if loginPersistentFlag {
		if configDir, err := os.UserConfigDir(); err != nil {
			return fmt.Errorf("could not save credentials persistent: %w", err)
		} else {
			os.MkdirAll(filepath.Join(configDir, "crunchy-cli"), 0755)
			if err = os.WriteFile(filepath.Join(configDir, "crunchy-cli", "crunchy"), []byte(etpRt), 0600); err != nil {
				return err
			}
			out.Info("The login information will be stored permanently UNENCRYPTED on your drive (%s)", filepath.Join(configDir, "crunchy-cli", "crunchy"))
		}
	}
	if err = os.WriteFile(filepath.Join(os.TempDir(), ".crunchy"), []byte(etpRt), 0600); err != nil {
		return err
	}

	if !loginPersistentFlag {
		out.Info("Due to security reasons, you have to login again on the next reboot")
	}

	return nil
}
