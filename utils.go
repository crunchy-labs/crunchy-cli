package crunchyroll

import (
	"encoding/json"
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
