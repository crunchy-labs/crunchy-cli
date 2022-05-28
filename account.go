package crunchyroll

// Account contains information about a crunchyroll account.
type Account struct {
	AccountID     string `json:"account_id"`
	ExternalID    string `json:"external_id"`
	EmailVerified bool   `json:"email_verified"`
	Created       string `json:"created"`

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
	Username                         string `json:"username"`
}
