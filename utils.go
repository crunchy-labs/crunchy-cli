package crunchyroll

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
)

func decodeMapToStruct(m interface{}, s interface{}) error {
	jsonBody, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBody, s)
}

func regexGroups(parsed [][]string, subexpNames ...string) map[string]string {
	groups := map[string]string{}
	for _, match := range parsed {
		for i, content := range match {
			if subexpName := subexpNames[i]; subexpName != "" {
				groups[subexpName] = content
			}
		}
	}
	return groups
}

func encodeStructToQueryValues(s interface{}) (string, error) {
	values := make(url.Values)
	v := reflect.ValueOf(s)

	for i := 0; i < v.Type().NumField(); i++ {

		// don't include parameters with default or without values in the query to avoid corruption of the API response.
		switch v.Field(i).Kind() {
		case reflect.Slice, reflect.String:
			if v.Field(i).Len() == 0 {
				continue
			}
		case reflect.Bool:
			if !v.Field(i).Bool() {
				continue
			}
		case reflect.Uint:
			if v.Field(i).Uint() == 0 {
				continue
			}
		}

		key := v.Type().Field(i).Tag.Get("param")
		var val string

		if v.Field(i).Kind() == reflect.Slice {
			var items []string

			for _, i := range v.Field(i).Interface().([]string) {
				items = append(items, i)
			}

			val = strings.Join(items, ",")
		} else {
			val = fmt.Sprint(v.Field(i).Interface())
		}

		values.Add(key, val)
	}

	return values.Encode(), nil
}
