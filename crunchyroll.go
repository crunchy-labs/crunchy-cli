package crunchyroll

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// LOCALE represents a locale / language.
type LOCALE string

const (
	JP LOCALE = "ja-JP"
	US        = "en-US"
	LA        = "es-419"
	ES        = "es-ES"
	FR        = "fr-FR"
	PT        = "pt-PT"
	BR        = "pt-BR"
	IT        = "it-IT"
	DE        = "de-DE"
	RU        = "ru-RU"
	AR        = "ar-SA"
)

// MediaType represents a media type.
type MediaType string

const (
	SERIES       MediaType = "series"
	MOVIELISTING           = "movie_listing"
)

// SortType represents a sort type.
type SortType string

const (
	POPULARITY   SortType = "popularity"
	NEWLYADDED            = "newly_added"
	ALPHABETICAL          = "alphabetical"
)

type Crunchyroll struct {
	// Client is the http.Client to perform all requests over.
	Client *http.Client
	// Context can be used to stop requests with Client and is context.Background by default.
	Context context.Context
	// Locale specifies in which language all results should be returned / requested.
	Locale LOCALE
	// EtpRt is the crunchyroll beta equivalent to a session id (prior SessionID field in
	// this struct in v2 and below).
	EtpRt string

	// Config stores parameters which are needed by some api calls.
	Config struct {
		TokenType   string
		AccessToken string

		Bucket string

		CountryCode    string
		Premium        bool
		Channel        string
		Policy         string
		Signature      string
		KeyPairID      string
		AccountID      string
		ExternalID     string
		MaturityRating string
	}

	// If cache is true, internal caching is enabled.
	cache bool
}

// BrowseOptions represents options for browsing the crunchyroll catalog.
type BrowseOptions struct {
	// Categories specifies the categories of the entries.
	Categories []string `param:"categories"`

	// IsDubbed specifies whether the entries should be dubbed.
	IsDubbed bool `param:"is_dubbed"`

	// IsSubbed specifies whether the entries should be subbed.
	IsSubbed bool `param:"is_subbed"`

	// Simulcast specifies a particular simulcast season by id in which the entries have been aired.
	Simulcast string `param:"season_tag"`

	// Sort specifies how the entries should be sorted.
	Sort SortType `param:"sort_by"`

	// Start specifies the index from which the entries should be returned.
	Start uint `param:"start"`

	// Type specifies the media type of the entries.
	Type MediaType `param:"type"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Country     string `json:"country"`
	AccountID   string `json:"account_id"`
}

// LoginWithCredentials logs in via crunchyroll username or email and password.
func LoginWithCredentials(user string, password string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	endpoint := "https://beta-api.crunchyroll.com/auth/v1/token"
	values := url.Values{}
	values.Set("username", user)
	values.Set("password", password)
	values.Set("grant_type", "password")

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic aHJobzlxM2F3dnNrMjJ1LXRzNWE6cHROOURteXRBU2Z6QjZvbXVsSzh6cUxzYTczVE1TY1k=")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := request(req, client)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var loginResp loginResponse
	json.NewDecoder(resp.Body).Decode(&loginResp)

	var etpRt string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "etp_rt" {
			etpRt = cookie.Value
			break
		}
	}

	return postLogin(loginResp, etpRt, locale, client)
}

// LoginWithSessionID logs in via a crunchyroll session id.
// Session ids are automatically generated as a cookie when visiting https://www.crunchyroll.com.
//
// Deprecated: Login via session id caused some trouble in the past (e.g. #15 or #30) which resulted in
// login not working. Use LoginWithEtpRt instead. EtpRt practically the crunchyroll beta equivalent to
// a session id.
// The method will stay in the library until session id login is removed completely or login with it
// does not work for a longer period of time.
func LoginWithSessionID(sessionID string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	endpoint := fmt.Sprintf("https://api.crunchyroll.com/start_session.0.json?session_id=%s",
		sessionID)
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to start session: %s", resp.Status)
	}

	var jsonBody map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse start session with session id response: %w", err)
	}
	if isError, ok := jsonBody["error"]; ok && isError.(bool) {
		return nil, fmt.Errorf("invalid session id (%s): %s", jsonBody["message"].(string), jsonBody["code"])
	}

	var etpRt string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "etp_rt" {
			etpRt = cookie.Value
			break
		}
	}

	return LoginWithEtpRt(etpRt, locale, client)
}

// LoginWithEtpRt logs in via the crunchyroll etp rt cookie. This cookie is the crunchyroll beta
// equivalent to the classic session id.
// The etp_rt cookie is automatically set when visiting https://beta.crunchyroll.com. Note that you
// need a crunchyroll account to access it.
func LoginWithEtpRt(etpRt string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	endpoint := "https://beta-api.crunchyroll.com/auth/v1/token"
	grantType := url.Values{}
	grantType.Set("grant_type", "etp_rt_cookie")

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(grantType.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "etp_rt",
		Value: etpRt,
	})
	resp, err := request(req, client)
	if err != nil {
		return nil, err
	}

	var loginResp loginResponse
	json.NewDecoder(resp.Body).Decode(&loginResp)

	return postLogin(loginResp, etpRt, locale, client)
}

func postLogin(loginResp loginResponse, etpRt string, locale LOCALE, client *http.Client) (*Crunchyroll, error) {
	crunchy := &Crunchyroll{
		Client:  client,
		Context: context.Background(),
		Locale:  locale,
		EtpRt:   etpRt,
		cache:   true,
	}

	crunchy.Config.TokenType = loginResp.TokenType
	crunchy.Config.AccessToken = loginResp.AccessToken
	crunchy.Config.AccountID = loginResp.AccountID
	crunchy.Config.CountryCode = loginResp.Country

	var jsonBody map[string]any

	endpoint := "https://beta-api.crunchyroll.com/index/v2"
	resp, err := crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&jsonBody)
	
	cms := jsonBody["cms"].(map[string]any)
	crunchy.Config.Bucket = strings.TrimPrefix(cms["bucket"].(string), "/")
	if strings.HasSuffix(crunchy.Config.Bucket, "crunchyroll") {
		crunchy.Config.Premium = true
		crunchy.Config.Channel = "crunchyroll"
	} else {
		crunchy.Config.Premium = false
		crunchy.Config.Channel = "-"
	}

	if strings.Contains(cms["bucket"].(string), "crunchyroll") {
		crunchy.Config.Premium = true
		crunchy.Config.Channel = "crunchyroll"
	} else {
		crunchy.Config.Premium = false
		crunchy.Config.Channel = "-"
	}
  
	crunchy.Config.Policy = cms["policy"].(string)
	crunchy.Config.Signature = cms["signature"].(string)
	crunchy.Config.KeyPairID = cms["key_pair_id"].(string)

	endpoint = "https://beta-api.crunchyroll.com/accounts/v1/me"
	resp, err = crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&jsonBody)
	crunchy.Config.ExternalID = jsonBody["external_id"].(string)

	endpoint = "https://beta-api.crunchyroll.com/accounts/v1/me/profile"
	resp, err = crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&jsonBody)
	crunchy.Config.MaturityRating = jsonBody["maturity_rating"].(string)

	return crunchy, nil
}

func request(req *http.Request, client *http.Client) (*http.Response, error) {
	resp, err := client.Do(req)
	if err == nil {
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		defer resp.Body.Close()
		defer func() {
			resp.Body = io.NopCloser(&buf)
		}()

		if buf.Len() != 0 {
			var errMap map[string]any

			if err = json.Unmarshal(buf.Bytes(), &errMap); err != nil {
				return nil, fmt.Errorf("invalid json response: %w", err)
			}

			if val, ok := errMap["error"]; ok {
				if errorAsString, ok := val.(string); ok {
					if code, ok := errMap["code"].(string); ok {
						return nil, fmt.Errorf("error for endpoint %s (%d): %s - %s", req.URL.String(), resp.StatusCode, errorAsString, code)
					}
					return nil, fmt.Errorf("error for endpoint %s (%d): %s", req.URL.String(), resp.StatusCode, errorAsString)
				} else if errorAsBool, ok := val.(bool); ok && errorAsBool {
					if msg, ok := errMap["message"].(string); ok {
						return nil, fmt.Errorf("error for endpoint %s (%d): %s", req.URL.String(), resp.StatusCode, msg)
					}
				}
			}
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("error for endpoint %s: %s", req.URL.String(), resp.Status)
		}
	}
	return resp, err
}

// request is a base function which handles api requests.
func (c *Crunchyroll) request(endpoint string, method string) (*http.Response, error) {
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("%s %s", c.Config.TokenType, c.Config.AccessToken))

	return request(req, c.Client)
}

// IsCaching returns if data gets cached or not.
// See SetCaching for more information.
func (c *Crunchyroll) IsCaching() bool {
	return c.cache
}

// SetCaching enables or disables internal caching of requests made.
// Caching is enabled by default.
// If it is disabled the already cached data still gets called.
// The best way to prevent this is to create a complete new Crunchyroll struct.
func (c *Crunchyroll) SetCaching(caching bool) {
	c.cache = caching
}

// Search searches a query and returns all found series and movies within the given limit.
func (c *Crunchyroll) Search(query string, limit uint) (s []*Series, m []*Movie, err error) {
	searchEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/search?q=%s&n=%d&type=&locale=%s",
		query, limit, c.Locale)
	resp, err := c.request(searchEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'search' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		item := item.(map[string]interface{})
		if item["total"].(float64) > 0 {
			switch item["type"] {
			case "series":
				for _, series := range item["items"].([]interface{}) {
					series2 := &Series{
						crunchy: c,
					}
					if err := decodeMapToStruct(series, series2); err != nil {
						return nil, nil, err
					}
					if err := decodeMapToStruct(series.(map[string]interface{})["series_metadata"].(map[string]interface{}), series2); err != nil {
						return nil, nil, err
					}

					s = append(s, series2)
				}
			case "movie_listing":
				for _, movie := range item["items"].([]interface{}) {
					movie2 := &Movie{
						crunchy: c,
					}
					if err := decodeMapToStruct(movie, movie2); err != nil {
						return nil, nil, err
					}

					m = append(m, movie2)
				}
			}
		}
	}

	return s, m, nil
}

// FindVideoByName finds a Video (Season or Movie) by its name.
// Use this in combination with ParseVideoURL and hand over the corresponding results
// to this function.
//
// Deprecated: Use Search instead. The first result sometimes isn't the correct one
// so this function is inaccurate in some cases.
// See https://github.com/ByteDream/crunchyroll-go/issues/22 for more information.
func (c *Crunchyroll) FindVideoByName(seriesName string) (Video, error) {
	s, m, err := c.Search(seriesName, 1)
	if err != nil {
		return nil, err
	}

	if len(s) > 0 {
		return s[0], nil
	} else if len(m) > 0 {
		return m[0], nil
	}
	return nil, fmt.Errorf("no series or movie could be found")
}

// FindEpisodeByName finds an episode by its crunchyroll series name and episode title.
// Use this in combination with ParseEpisodeURL and hand over the corresponding results
// to this function.
func (c *Crunchyroll) FindEpisodeByName(seriesName, episodeTitle string) ([]*Episode, error) {
	series, _, err := c.Search(seriesName, 5)
	if err != nil {
		return nil, err
	}

	var matchingEpisodes []*Episode
	for _, s := range series {
		seasons, err := s.Seasons()
		if err != nil {
			return nil, err
		}

		for _, season := range seasons {
			episodes, err := season.Episodes()
			if err != nil {
				return nil, err
			}
			for _, episode := range episodes {
				if episode.SlugTitle == episodeTitle {
					matchingEpisodes = append(matchingEpisodes, episode)
				}
			}
		}
	}

	return matchingEpisodes, nil
}

// ParseVideoURL tries to extract the crunchyroll series / movie name out of the given url.
//
// Deprecated: Crunchyroll classic urls are sometimes not safe to use, use ParseBetaSeriesURL
// if possible since beta url are always safe to use.
// The method will stay in the library until only beta urls are supported by crunchyroll itself.
func ParseVideoURL(url string) (seriesName string, ok bool) {
	pattern := regexp.MustCompile(`(?m)^https?://(www\.)?crunchyroll\.com(/\w{2}(-\w{2})?)?/(?P<series>[^/]+)(/videos)?/?$`)
	if urlMatch := pattern.FindAllStringSubmatch(url, -1); len(urlMatch) != 0 {
		groups := regexGroups(urlMatch, pattern.SubexpNames()...)
		seriesName = groups["series"]

		if seriesName != "" {
			ok = true
		}
	}
	return
}

// ParseEpisodeURL tries to extract the crunchyroll series name, title, episode number and web id out of the given crunchyroll url
// Note that the episode number can be misleading. For example if an episode has the episode number 23.5 (slime isekai)
// the episode number will be 235.
//
// Deprecated: Crunchyroll classic urls are sometimes not safe to use, use ParseBetaEpisodeURL
// if possible since beta url are always safe to use.
// The method will stay in the library until only beta urls are supported by crunchyroll itself.
func ParseEpisodeURL(url string) (seriesName, title string, episodeNumber int, webId int, ok bool) {
	pattern := regexp.MustCompile(`(?m)^https?://(www\.)?crunchyroll\.com(/\w{2}(-\w{2})?)?/(?P<series>[^/]+)/episode-(?P<number>\d+)-(?P<title>.+)-(?P<webId>\d+).*`)
	if urlMatch := pattern.FindAllStringSubmatch(url, -1); len(urlMatch) != 0 {
		groups := regexGroups(urlMatch, pattern.SubexpNames()...)
		seriesName = groups["series"]
		episodeNumber, _ = strconv.Atoi(groups["number"])
		title = groups["title"]
		webId, _ = strconv.Atoi(groups["webId"])

		if seriesName != "" && title != "" && webId != 0 {
			ok = true
		}
	}
	return
}

// ParseBetaSeriesURL tries to extract the season id of the given crunchyroll beta url, pointing to a season.
func ParseBetaSeriesURL(url string) (seasonId string, ok bool) {
	pattern := regexp.MustCompile(`(?m)^https?://(www\.)?beta\.crunchyroll\.com/(\w{2}/)?series/(?P<seasonId>\w+).*`)
	if urlMatch := pattern.FindAllStringSubmatch(url, -1); len(urlMatch) != 0 {
		groups := regexGroups(urlMatch, pattern.SubexpNames()...)
		seasonId = groups["seasonId"]
		ok = true
	}
	return
}

// ParseBetaEpisodeURL tries to extract the episode id of the given crunchyroll beta url, pointing to an episode.
func ParseBetaEpisodeURL(url string) (episodeId string, ok bool) {
	pattern := regexp.MustCompile(`(?m)^https?://(www\.)?beta\.crunchyroll\.com/(\w{2}/)?watch/(?P<episodeId>\w+).*`)
	if urlMatch := pattern.FindAllStringSubmatch(url, -1); len(urlMatch) != 0 {
		groups := regexGroups(urlMatch, pattern.SubexpNames()...)
		episodeId = groups["episodeId"]
		ok = true
	}
	return
}

// Browse browses the crunchyroll catalog filtered by the specified options and returns all found series and movies within the given limit.
func (c *Crunchyroll) Browse(options BrowseOptions, limit uint) (s []*Series, m []*Movie, err error) {
	query, err := encodeStructToQueryValues(options)
	if err != nil {
		return nil, nil, err
	}

	browseEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/browse?%s&n=%d&locale=%s",
		query, limit, c.Locale)
	resp, err := c.request(browseEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'browse' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		switch item.(map[string]interface{})["type"] {
		case "series":
			series := &Series{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, series); err != nil {
				return nil, nil, err
			}
			if err := decodeMapToStruct(item.(map[string]interface{})["series_metadata"].(map[string]interface{}), series); err != nil {
				return nil, nil, err
			}

			s = append(s, series)
		case "movie_listing":
			movie := &Movie{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, movie); err != nil {
				return nil, nil, err
			}

			m = append(m, movie)
		}
	}

	return s, m, nil
}

// Categories returns all available categories and possible subcategories.
func (c *Crunchyroll) Categories(includeSubcategories bool) (ca []*Category, err error) {
	tenantCategoriesEndpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/tenant_categories?include_subcategories=%t&locale=%s",
		includeSubcategories, c.Locale)
	resp, err := c.request(tenantCategoriesEndpoin, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'tenant_categories' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		category := &Category{}
		if err := decodeMapToStruct(item, category); err != nil {
			return nil, err
		}

		ca = append(ca, category)
	}

	return ca, nil
}

// Simulcasts returns all available simulcast seasons for the current locale.
func (c *Crunchyroll) Simulcasts() (s []*Simulcast, err error) {
	seasonListEndpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/season_list?locale=%s", c.Locale)
	resp, err := c.request(seasonListEndpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'season_list' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		simulcast := &Simulcast{}
		if err := decodeMapToStruct(item, simulcast); err != nil {
			return nil, err
		}

		s = append(s, simulcast)
	}

	return s, nil
}

// News returns the top and latest news from crunchyroll for the current locale within the given limits.
func (c *Crunchyroll) News(topLimit uint, latestLimit uint) (t []*News, l []*News, err error) {
	newsFeedEndpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/news_feed?top_news_n=%d&latest_news_n=%d&locale=%s",
		topLimit, latestLimit, c.Locale)
	resp, err := c.request(newsFeedEndpoint, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'news_feed' response: %w", err)
	}

	topNews := jsonBody["top_news"].(map[string]interface{})
	for _, item := range topNews["items"].([]interface{}) {
		topNews := &News{}
		if err := decodeMapToStruct(item, topNews); err != nil {
			return nil, nil, err
		}

		t = append(t, topNews)
	}

	latestNews := jsonBody["latest_news"].(map[string]interface{})
	for _, item := range latestNews["items"].([]interface{}) {
		latestNews := &News{}
		if err := decodeMapToStruct(item, latestNews); err != nil {
			return nil, nil, err
		}

		l = append(l, latestNews)
	}

	return t, l, nil
}

// Recommendations returns series and movie recommendations from crunchyroll based on the currently logged in account within the given limit.
func (c *Crunchyroll) Recommendations(limit uint) (s []*Series, m []*Movie, err error) {
	recommendationsEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/%s/recommendations?n=%d&locale=%s",
		c.Config.AccountID, limit, c.Locale)
	resp, err := c.request(recommendationsEndpoint)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'recommendations' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		switch item.(map[string]interface{})["type"] {
		case "series":
			series := &Series{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, series); err != nil {
				return nil, nil, err
			}
			if err := decodeMapToStruct(item.(map[string]interface{})["series_metadata"].(map[string]interface{}), series); err != nil {
				return nil, nil, err
			}

			s = append(s, series)
		case "movie_listing":
			movie := &Movie{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, movie); err != nil {
				return nil, nil, err
			}

			m = append(m, movie)
		}
	}

	return s, m, nil
}

// UpNext returns the episodes that are up next based on the currently logged in account within the given limit.
func (c *Crunchyroll) UpNext(limit uint) (e []*Episode, err error) {
	upNextAccountEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/%s/up_next_account?n=%d&locale=%s",
		c.Config.AccountID, limit, c.Locale)
	resp, err := c.request(upNextAccountEndpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'up_next_account' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		panel := item.(map[string]interface{})["panel"]

		episode := &Episode{
			crunchy: c,
		}
		if err := decodeMapToStruct(panel, episode); err != nil {
			return nil, err
		}

		e = append(e, episode)
	}

	return e, nil
}

// SimilarTo returns similar series and movies according to crunchyroll to the one specified by id within the given limit.
func (c *Crunchyroll) SimilarTo(id string, limit uint) (s []*Series, m []*Movie, err error) {
	similarToEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/%s/similar_to?guid=%s&n=%d&locale=%s",
		c.Config.AccountID, id, limit, c.Locale)
	resp, err := c.request(similarToEndpoint)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, nil, fmt.Errorf("failed to parse 'similar_to' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		switch item.(map[string]interface{})["type"] {
		case "series":
			series := &Series{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, series); err != nil {
				return nil, nil, err
			}
			if err := decodeMapToStruct(item.(map[string]interface{})["series_metadata"].(map[string]interface{}), series); err != nil {
				return nil, nil, err
			}

			s = append(s, series)
		case "movie_listing":
			movie := &Movie{
				crunchy: c,
			}
			if err := decodeMapToStruct(item, movie); err != nil {
				return nil, nil, err
			}

			m = append(m, movie)
		}
	}

	return s, m, nil
}

// WatchHistory returns the history of watched episodes based on the currently logged in account from the given page with the given size.
func (c *Crunchyroll) WatchHistory(page uint, size uint) (e []*HistoryEpisode, err error) {
	watchHistoryEndpoint := fmt.Sprintf("https://beta-api.crunchyroll.com/content/v1/watch-history/%s?page=%d&page_size=%d&locale=%s",
		c.Config.AccountID, page, size, c.Locale)
	resp, err := c.request(watchHistoryEndpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, fmt.Errorf("failed to parse 'watch-history' response: %w", err)
	}

	for _, item := range jsonBody["items"].([]interface{}) {
		panel := item.(map[string]interface{})["panel"]

		episode := &Episode{
			crunchy: c,
		}
		if err := decodeMapToStruct(panel, episode); err != nil {
			return nil, err
		}

		historyEpisode := &HistoryEpisode{
			Episode: episode,
		}
		if err := decodeMapToStruct(item, historyEpisode); err != nil {
			return nil, err
		}

		e = append(e, historyEpisode)
	}

	return e, nil
}

// Account returns information about the currently logged in crunchyroll account.
func (c *Crunchyroll) Account() (*Account, error) {
	resp, err := c.request("https://beta.crunchyroll.com/accounts/v1/me")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	account := &Account{}

	if err = json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, fmt.Errorf("failed to parse 'me' response: %w", err)
	}

	resp, err = c.request("https://beta.crunchyroll.com/accounts/v1/me/profile")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, fmt.Errorf("failed to parse 'profile' response: %w", err)
	}

	return account, nil
}
