package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Categories returns all available categories and possible subcategories.
func (c *Crunchyroll) Categories(includeSubcategories bool) (ca []*Category, err error) {
	tenantCategoriesEndpoint := fmt.Sprintf("https://beta.crunchyroll.com/content/v1/tenant_categories?include_subcategories=%t&locale=%s",
		includeSubcategories, c.Locale)
	resp, err := c.request(tenantCategoriesEndpoint, http.MethodGet)
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

// Category contains all information about a category.
type Category struct {
	Category string `json:"tenant_category"`

	SubCategories []struct {
		Category       string `json:"tenant_category"`
		ParentCategory string `json:"parent_category"`

		Localization struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Locale      LOCALE `json:"locale"`
		} `json:"localization"`

		Slug string `json:"slug"`
	} `json:"sub_categories"`

	Images struct {
		Background []Image `json:"background"`
		Low        []Image `json:"low"`
	} `json:"images"`

	Localization struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Locale      LOCALE `json:"locale"`
	} `json:"localization"`

	Slug string `json:"slug"`
}
