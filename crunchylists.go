package crunchyroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type CrunchyLists struct {
	crunchy *Crunchyroll

	Items        []*CrunchyListPreview `json:"items"`
	TotalPublic  int                   `json:"total_public"`
	TotalPrivate int                   `json:"total_private"`
	MaxPrivate   int                   `json:"max_private"`
}

func (cl *CrunchyLists) Create(name string) (*CrunchyList, error) {
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

	return CrunchyListFromID(cl.crunchy, jsonBody["list_id"].(string))
}

type CrunchyListPreview struct {
	crunchy *Crunchyroll

	ListID     string    `json:"list_id"`
	IsPublic   bool      `json:"is_public"`
	Total      int       `json:"total"`
	ModifiedAt time.Time `json:"modified_at"`
	Title      string    `json:"title"`
}

func (clp *CrunchyListPreview) CrunchyList() (*CrunchyList, error) {
	return CrunchyListFromID(clp.crunchy, clp.ListID)
}

func CrunchyListFromID(crunchy *Crunchyroll, id string) (*CrunchyList, error) {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s?locale=%s", crunchy.Config.AccountID, id, crunchy.Locale)
	resp, err := crunchy.request(endpoint, http.MethodGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	crunchyList := &CrunchyList{
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

type CrunchyList struct {
	crunchy *Crunchyroll

	ID string `json:"id"`

	Max        int                `json:"max"`
	Total      int                `json:"total"`
	Title      string             `json:"title"`
	IsPublic   bool               `json:"is_public"`
	ModifiedAt time.Time          `json:"modified_at"`
	Items      []*CrunchyListItem `json:"items"`
}

func (cl *CrunchyList) AddSeries(series *Series) error {
	return cl.AddSeriesFromID(series.ID)
}

func (cl *CrunchyList) AddSeriesFromID(id string) error {
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

func (cl *CrunchyList) RemoveSeries(series *Series) error {
	return cl.RemoveSeriesFromID(series.ID)
}

func (cl *CrunchyList) RemoveSeriesFromID(id string) error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s/%s?locale=%s", cl.crunchy.Config.AccountID, cl.ID, id, cl.crunchy.Locale)
	_, err := cl.crunchy.request(endpoint, http.MethodDelete)
	return err
}

func (cl *CrunchyList) Delete() error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s?locale=%s", cl.crunchy.Config.AccountID, cl.ID, cl.crunchy.Locale)
	_, err := cl.crunchy.request(endpoint, http.MethodDelete)
	return err
}

func (cl *CrunchyList) Rename(name string) error {
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

type CrunchyListItem struct {
	crunchy *Crunchyroll

	ListID     string    `json:"list_id"`
	ID         string    `json:"id"`
	ModifiedAt time.Time `json:"modified_at"`
	Panel      Panel     `json:"panel"`
}

func (cli *CrunchyListItem) Remove() error {
	endpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/custom-lists/%s/%s/%s", cli.crunchy.Config.AccountID, cli.ListID, cli.ID)
	_, err := cli.crunchy.request(endpoint, http.MethodDelete)
	return err
}
