package crunchyroll

import (
	"crypto/cipher"
	"encoding/json"
	"github.com/grafov/m3u8"
	"io/ioutil"
	"net/http"
)

func decodeMapToStruct(m interface{}, s interface{}) error {
	jsonBody, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBody, s)
}

// https://github.com/oopsguy/m3u8/blob/4150e93ec8f4f8718875a02973f5d792648ecb97/tool/crypt.go#L25
func decryptSegment(client *http.Client, segment *m3u8.MediaSegment, block cipher.Block, iv []byte) ([]byte, error) {
	resp, err := client.Get(segment.URI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	blockMode := cipher.NewCBCDecrypter(block, iv[:block.BlockSize()])
	decrypted := make([]byte, len(raw))
	blockMode.CryptBlocks(decrypted, raw)
	raw = pkcs5UnPadding(decrypted)

	return raw, nil
}

// https://github.com/oopsguy/m3u8/blob/4150e93ec8f4f8718875a02973f5d792648ecb97/tool/crypt.go#L47
func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
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
