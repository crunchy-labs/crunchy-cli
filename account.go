package crunchyroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Account returns information about the currently logged in crunchyroll account.
func (c *Crunchyroll) Account() (*Account, error) {
	resp, err := c.request("https://beta.crunchyroll.com/accounts/v1/me", http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	account := &Account{
		crunchy: c,
	}

	if err = json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, fmt.Errorf("failed to parse 'me' response: %w", err)
	}

	resp, err = c.request("https://beta.crunchyroll.com/accounts/v1/me/profile", http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, fmt.Errorf("failed to parse 'profile' response: %w", err)
	}

	return account, nil
}

// Account contains information about a crunchyroll account.
type Account struct {
	crunchy *Crunchyroll

	AccountID     string    `json:"account_id"`
	ExternalID    string    `json:"external_id"`
	EmailVerified bool      `json:"email_verified"`
	Created       time.Time `json:"created"`

	Avatar                           string `json:"avatar"`
	CrBetaOptIn                      bool   `json:"cr_beta_opt_in"`
	Email                            string `json:"email"`
	MatureContentFlagManga           string `json:"mature_content_flag_manga"`
	MaturityRating                   string `json:"maturity_rating"`
	OptOutAndroidInAppMarketing      bool   `json:"opt_out_android_in_app_marketing"`
	OptOutFreeTrials                 bool   `json:"opt_out_free_trials"`
	OptOutNewMediaQueueUpdates       bool   `json:"opt_out_new_media_queue_updates"`
	OptOutNewsletters                bool   `json:"opt_out_newsletters"`
	OptOutPmUpdates                  bool   `json:"opt_out_pm_updates"`
	OptOutPromotionalUpdates         bool   `json:"opt_out_promotional_updates"`
	OptOutQueueUpdates               bool   `json:"opt_out_queue_updates"`
	OptOutStoreDeals                 bool   `json:"opt_out_store_deals"`
	PreferredCommunicationLanguage   LOCALE `json:"preferred_communication_language"`
	PreferredContentSubtitleLanguage LOCALE `json:"preferred_content_subtitle_language"`
	QaUser                           bool   `json:"qa_user"`

	Username  string     `json:"username"`
	Wallpaper *Wallpaper `json:"wallpaper"`
}

// UpdatePreferredEmailLanguage sets in which language emails should be received.
func (a *Account) UpdatePreferredEmailLanguage(language LOCALE) error {
	err := a.updatePreferences("preferred_communication_language", string(language))
	if err == nil {
		a.PreferredCommunicationLanguage = language
	}
	return err
}

// UpdatePreferredVideoSubtitleLanguage sets in which language default subtitles should be shown
func (a *Account) UpdatePreferredVideoSubtitleLanguage(language LOCALE) error {
	err := a.updatePreferences("preferred_content_subtitle_language", string(language))
	if err == nil {
		a.PreferredContentSubtitleLanguage = language
	}
	return err
}

// UpdateMatureVideoContent sets if mature video content / 18+ content should be shown
func (a *Account) UpdateMatureVideoContent(enabled bool) error {
	if enabled {
		return a.updatePreferences("maturity_rating", "M3")
	} else {
		return a.updatePreferences("maturity_rating", "M2")
	}
}

// UpdateMatureMangaContent sets if mature manga content / 18+ content should be shown
func (a *Account) UpdateMatureMangaContent(enabled bool) error {
	if enabled {
		return a.updatePreferences("mature_content_flag_manga", "1")
	} else {
		return a.updatePreferences("mature_content_flag_manga", "0")
	}
}

func (a *Account) updatePreferences(name, value string) error {
	endpoint := "https://beta.crunchyroll.com/accounts/v1/me/profile"
	body, _ := json.Marshal(map[string]string{name: value})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = a.crunchy.requestFull(req)
	return err
}

// ChangePassword changes the password for the current account.
func (a *Account) ChangePassword(currentPassword, newPassword string) error {
	endpoint := "https://beta.crunchyroll.com/accounts/v1/me/credentials"
	body, _ := json.Marshal(map[string]string{"accountId": a.AccountID, "current_password": currentPassword, "new_password": newPassword})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = a.crunchy.requestFull(req)
	return err
}

// ChangeEmail changes the email address for the current account.
func (a *Account) ChangeEmail(currentPassword, newEmail string) error {
	endpoint := "https://beta.crunchyroll.com/accounts/v1/me/credentials"
	body, _ := json.Marshal(map[string]string{"current_password": currentPassword, "email": newEmail})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = a.crunchy.requestFull(req)
	return err
}

// AvailableWallpapers returns all available wallpapers which can be set as profile wallpaper.
func (a *Account) AvailableWallpapers() (w []*Wallpaper, err error) {
	endpoint := "https://beta.crunchyroll.com/assets/v1/wallpaper"
	resp, err := a.crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]any
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	err = decodeMapToStruct(jsonBody["items"].([]any), &w)
	return
}

// ChangeWallpaper changes the profile wallpaper of the current user. Use AvailableWallpapers
// to get all available ones.
func (a *Account) ChangeWallpaper(wallpaper *Wallpaper) error {
	endpoint := "https://beta.crunchyroll.com/accounts/v1/me/profile"
	body, _ := json.Marshal(map[string]string{"wallpaper": string(*wallpaper)})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	_, err = a.crunchy.requestFull(req)
	return err
}
