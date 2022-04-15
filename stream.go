package crunchyroll

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafov/m3u8"
	"regexp"
)

// Stream contains information about all available video stream of an episode.
type Stream struct {
	crunchy *Crunchyroll

	children []*Format

	HardsubLocale LOCALE
	AudioLocale   LOCALE
	Subtitles     []*Subtitle

	formatType FormatType
	id         string
	streamURL  string
}

// StreamsFromID returns a stream by its api id.
func StreamsFromID(crunchy *Crunchyroll, id string) ([]*Stream, error) {
	return fromVideoStreams(crunchy, fmt.Sprintf("https://beta-api.crunchyroll.com/cms/v2/%s/%s/%s/videos/%s/streams?locale=%s&Signature=%s&Policy=%s&Key-Pair-Id=%s",
		crunchy.Config.CountryCode,
		crunchy.Config.MaturityRating,
		crunchy.Config.Channel,
		id,
		crunchy.Locale,
		crunchy.Config.Signature,
		crunchy.Config.Policy,
		crunchy.Config.KeyPairID))
}

// Formats returns all formats which are available for the stream.
func (s *Stream) Formats() ([]*Format, error) {
	if s.children != nil {
		return s.children, nil
	}

	resp, err := s.crunchy.Client.Get(s.streamURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	playlist, _, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return nil, err
	}
	var formats []*Format
	for _, variant := range playlist.(*m3u8.MasterPlaylist).Variants {
		formats = append(formats, &Format{
			crunchy:     s.crunchy,
			ID:          s.id,
			FormatType:  s.formatType,
			Video:       variant,
			AudioLocale: s.AudioLocale,
			Hardsub:     s.HardsubLocale,
			Subtitles:   s.Subtitles,
		})
	}

	if s.crunchy.cache {
		s.children = formats
	}
	return formats, nil
}

// fromVideoStreams returns all streams which are accessible via the endpoint.
func fromVideoStreams(crunchy *Crunchyroll, endpoint string) (streams []*Stream, err error) {
	resp, err := crunchy.request(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var jsonBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&jsonBody)

	if len(jsonBody) == 0 {
		// this may get thrown when the crunchyroll account has just a normal account and not one with premium
		return nil, errors.New("no stream available")
	}

	audioLocale := jsonBody["audio_locale"].(string)

	var subtitles []*Subtitle
	for _, rawSubtitle := range jsonBody["subtitles"].(map[string]interface{}) {
		subtitle := &Subtitle{
			crunchy: crunchy,
		}
		decodeMapToStruct(rawSubtitle.(map[string]interface{}), subtitle)
		subtitles = append(subtitles, subtitle)
	}

	for _, streamData := range jsonBody["streams"].(map[string]interface{})["adaptive_hls"].(map[string]interface{}) {
		streamData := streamData.(map[string]interface{})

		hardsubLocale := streamData["hardsub_locale"].(string)

		var id string
		var formatType FormatType
		href := jsonBody["__links__"].(map[string]interface{})["resource"].(map[string]interface{})["href"].(string)
		if match := regexp.MustCompile(`(?sm)^/cms/v2/\S+/crunchyroll/(\w+)/(\w+)$`).FindAllStringSubmatch(href, -1); len(match) > 0 {
			formatType = FormatType(match[0][1])
			id = match[0][2]
		}

		stream := &Stream{
			crunchy:       crunchy,
			HardsubLocale: LOCALE(hardsubLocale),
			formatType:    formatType,
			id:            id,
			streamURL:     streamData["url"].(string),
			AudioLocale:   LOCALE(audioLocale),
			Subtitles:     subtitles,
		}

		streams = append(streams, stream)
	}

	return
}
