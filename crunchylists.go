package crunchyroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Crunchylists returns a struct to control crunchylists.
func (c *Crunchyroll) Crunchylists() (*Crunchylists, error) {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s?locale=%s", c.Config.AccountID, c.Locale)
	resp, err := c.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	crunchylists := &Crunchylists{
		crunchy: c,
	}
	json.NewDecoder(resp.Body).Decode(crunchylists)
	for _, item := range crunchylists.Items {
		item.crunchy = c
	}

	return crunchylists, nil
}

type Crunchylists struct {
	crunchy *Crunchyroll

	Items        []*CrunchylistPreview `json:"items"`
	TotalPublic  int                   `json:"total_public"`
	TotalPrivate int                   `json:"total_private"`
	MaxPrivate   int                   `json:"max_private"`
}

// Create creates a new crunchylist with the given name. Duplicate names for lists are allowed.
func (cl *Crunchylists) Create(name string) (*Crunchylist, error) {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s?locale=%s", cl.crunchy.Config.AccountID, cl.crunchy.Locale)
	body, _ := json.Marshal(map[string]string{"title": name})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := cl.crunchy.requestFull(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	return crunchylistFromID(cl.crunchy, jsonBody["list_id"].(string))
}

type CrunchylistPreview struct {
	crunchy *Crunchyroll

	ListID     string    `json:"list_id"`
	IsPublic   bool      `json:"is_public"`
	Total      int       `json:"total"`
	ModifiedAt time.Time `json:"modified_at"`
	Title      string    `json:"title"`
}

// Crunchylist returns the belonging Crunchylist struct.
func (clp *CrunchylistPreview) Crunchylist() (*Crunchylist, error) {
	return crunchylistFromID(clp.crunchy, clp.ListID)
}

type Crunchylist struct {
	crunchy *Crunchyroll

	ID string `json:"id"`

	Max        int                `json:"max"`
	Total      int                `json:"total"`
	Title      string             `json:"title"`
	IsPublic   bool               `json:"is_public"`
	ModifiedAt time.Time          `json:"modified_at"`
	Items      []*CrunchylistItem `json:"items"`
}

// AddSeries adds a series.
func (cl *Crunchylist) AddSeries(series *Series) error {
	return cl.AddSeriesFromID(series.ID)
}

// AddSeriesFromID adds a series from its id
func (cl *Crunchylist) AddSeriesFromID(id string) error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s?locale=%s", cl.crunchy.Config.AccountID, cl.ID, cl.crunchy.Locale)
	body, _ := json.Marshal(map[string]string{"content_id": id})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = cl.crunchy.requestFull(req)
	return err
}

// RemoveSeries removes a series
func (cl *Crunchylist) RemoveSeries(series *Series) error {
	return cl.RemoveSeriesFromID(series.ID)
}

// RemoveSeriesFromID removes a series by its id
func (cl *Crunchylist) RemoveSeriesFromID(id string) error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s/%s?locale=%s", cl.crunchy.Config.AccountID, cl.ID, id, cl.crunchy.Locale)
	_, err := cl.crunchy.request(endpoint, http.MethodDelete)
	return err
}

// Delete deleted the current crunchylist.
func (cl *Crunchylist) Delete() error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s?locale=%s", cl.crunchy.Config.AccountID, cl.ID, cl.crunchy.Locale)
	_, err := cl.crunchy.request(endpoint, http.MethodDelete)
	return err
}

// Rename renames the current crunchylist.
func (cl *Crunchylist) Rename(name string) error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s?locale=%s", cl.crunchy.Config.AccountID, cl.ID, cl.crunchy.Locale)
	body, _ := json.Marshal(map[string]string{"title": name})
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	_, err = cl.crunchy.requestFull(req)
	if err == nil {
		cl.Title = name
	}
	return err
}

func crunchylistFromID(crunchy *Crunchyroll, id string) (*Crunchylist, error) {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s?locale=%s", crunchy.Config.AccountID, id, crunchy.Locale)
	resp, err := crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	crunchyList := &Crunchylist{
		crunchy: crunchy,
		ID:      id,
	}
	if err := json.NewDecoder(resp.Body).Decode(crunchyList); err != nil {
		return nil, err
	}
	for _, item := range crunchyList.Items {
		item.crunchy = crunchy
	}
	return crunchyList, nil
}

type CrunchylistItem struct {
	crunchy *Crunchyroll

	ListID     string    `json:"list_id"`
	ID         string    `json:"id"`
	ModifiedAt time.Time `json:"modified_at"`
	Panel      Panel     `json:"panel"`
}

// Remove removes the current item from its crunchylist.
func (cli *CrunchylistItem) Remove() error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s/%s", cli.crunchy.Config.AccountID, cli.ListID, cli.ID)
	_, err := cli.crunchy.request(endpoint, http.MethodDelete)
	return err
}
