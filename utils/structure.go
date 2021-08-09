package utils

import (
	"errors"
	"github.com/ByteDream/crunchyroll"
	"sort"
	"sync"
)

// FormatStructure is the basic structure which every other structure implements.
// With it, and all other structures the api usage can be simplified magnificent
type FormatStructure struct {
	// initState is true if every format, stream, ... in the structure tree is initialized
	initState bool

	// getFunc specified the function which will be called if crunchyroll.Format is empty / not initialized yet.
	// It returns the formats itself, the parent streams (might be nil) and an error if one occurs
	getFunc func() ([]*crunchyroll.Format, []*crunchyroll.Stream, error)
	// formats holds all formats which were given
	formats []*crunchyroll.Format
	// parents holds all parents which were given
	parents []*crunchyroll.Stream
}

func newFormatStructure(parentStructure *StreamStructure) *FormatStructure {
	return &FormatStructure{
		getFunc: func() (formats []*crunchyroll.Format, parents []*crunchyroll.Stream, err error) {
			streams, err := parentStructure.Streams()
			if err != nil {
				return
			}

			var wg sync.WaitGroup
			var lock sync.Mutex

			for _, stream := range streams {
				wg.Add(1)
				stream := stream
				go func() {
					defer wg.Done()
					f, err := stream.Formats()
					if err != nil {
						return
					}
					lock.Lock()
					defer lock.Unlock()
					for _, format := range f {
						formats = append(formats, format)
						parents = append(parents, stream)
					}
				}()
			}
			wg.Wait()
			return
		},
	}
}

// NewFormatStructure returns a new FormatStructure, based on the given formats
func NewFormatStructure(formats []*crunchyroll.Format) *FormatStructure {
	return &FormatStructure{
		getFunc: func() ([]*crunchyroll.Format, []*crunchyroll.Stream, error) {
			return formats, nil, nil
		},
	}
}

// Formats returns all stored formats
func (fs *FormatStructure) Formats() ([]*crunchyroll.Format, error) {
	var err error
	if fs.formats == nil {
		if fs.formats, fs.parents, err = fs.getFunc(); err != nil {
			return nil, err
		}
		fs.initState = true
	}
	return fs.formats, nil
}

// FormatParent returns the parent stream of a format (if present).
// If the format or parent is not stored, an error will be returned
func (fs *FormatStructure) FormatParent(format *crunchyroll.Format) (*crunchyroll.Stream, error) {
	formats, err := fs.Formats()
	if err != nil {
		return nil, err
	}

	if fs.parents == nil {
		return nil, errors.New("no parents are given")
	}

	for i, f := range formats {
		if f == format {
			return fs.parents[i], nil
		}
	}
	return nil, errors.New("given format could not be found")
}

// InitAll recursive requests all given information.
// All functions of FormatStructure or other structs in this file which are executed after this have a much lesser chance to return any error,
// so the error return value of these functions can be pretty safely ignored.
// This function should only be called if you need to the access to any function of FormatStructure which returns a crunchyroll.Format (or an array of it).
// Re-calling this method can lead to heavy problems (believe me, it caused a simple bug and i've tried to fix it for several hours).
// Check FormatStructure.InitAllState if you can call this method without causing bugs
func (fs *FormatStructure) InitAll() error {
	var err error
	if fs.formats, fs.parents, err = fs.getFunc(); err != nil {
		return err
	}
	fs.initState = true
	return nil
}

// InitAllState returns FormatStructure.InitAll or FormatStructure.Formats was called.
// If so, all errors which are returned by functions of structs in this file can be safely ignored
func (fs *FormatStructure) InitAllState() bool {
	return fs.initState
}

// AvailableLocales returns all available audio, subtitle and hardsub locales for all formats.
// If includeEmpty is given, locales with no value are included too
func (fs *FormatStructure) AvailableLocales(includeEmpty bool) (audioLocales []crunchyroll.LOCALE, subtitleLocales []crunchyroll.LOCALE, hardsubLocales []crunchyroll.LOCALE, err error) {
	var formats []*crunchyroll.Format
	if formats, err = fs.Formats(); err != nil {
		return
	}

	audioMap := map[crunchyroll.LOCALE]interface{}{}
	subtitleMap := map[crunchyroll.LOCALE]interface{}{}
	hardsubMap := map[crunchyroll.LOCALE]interface{}{}
	for _, format := range formats {
		// audio locale should always have a valid locale
		if includeEmpty || !includeEmpty && format.AudioLocale != "" {
			audioMap[format.AudioLocale] = nil
		}
		if format.Subtitles != nil {
			for _, subtitle := range format.Subtitles {
				if subtitle.Locale == "" && !includeEmpty {
					continue
				}
				subtitleMap[subtitle.Locale] = nil
			}
		}
		if includeEmpty || !includeEmpty && format.Hardsub != "" {
			hardsubMap[format.Hardsub] = nil
		}
	}

	for k := range audioMap {
		audioLocales = append(audioLocales, k)
	}
	for k := range subtitleMap {
		subtitleLocales = append(subtitleLocales, k)
	}
	for k := range hardsubMap {
		hardsubLocales = append(hardsubLocales, k)
	}
	return
}

// FilterFormatsByAudio returns all formats which have the given locale as their audio locale
func (fs *FormatStructure) FilterFormatsByAudio(locale crunchyroll.LOCALE) (f []*crunchyroll.Format, err error) {
	var formats []*crunchyroll.Format
	if formats, err = fs.Formats(); err != nil {
		return nil, err
	}
	for _, format := range formats {
		if format.AudioLocale == locale {
			f = append(f, format)
		}
	}
	return
}

// FilterFormatsBySubtitle returns all formats which have the given locale as their subtitle locale.
// Hardsub indicates if the subtitle should be shown on the video itself
func (fs *FormatStructure) FilterFormatsBySubtitle(locale crunchyroll.LOCALE, hardsub bool) (f []*crunchyroll.Format, err error) {
	var formats []*crunchyroll.Format
	if formats, err = fs.Formats(); err != nil {
		return nil, err
	}
	for _, format := range formats {
		if hardsub && format.Hardsub == locale {
			f = append(f, format)
		} else if !hardsub && format.Hardsub == "" {
			f = append(f, format)
		}
	}
	return
}

// FilterFormatsByLocales returns all formats which have the given locales as their property.
// Hardsub is the same as in FormatStructure.FilterFormatsBySubtitle
func (fs *FormatStructure) FilterFormatsByLocales(audioLocale, subtitleLocale crunchyroll.LOCALE, hardsub bool) ([]*crunchyroll.Format, error) {
	var f []*crunchyroll.Format

	formats, err := fs.Formats()
	if err != nil {
		return nil, err
	}
	for _, format := range formats {
		if format.AudioLocale == audioLocale {
			if hardsub && format.Hardsub == subtitleLocale {
				f = append(f, format)
			} else if !hardsub && format.Hardsub == "" {
				f = append(f, format)
			}
		}
	}
	if len(f) == 0 {
		return nil, errors.New("could not find any matching format")
	}
	return f, nil
}

// OrderFormatsByID loops through all stored formats and returns a 2d slice
// where a row represents an id and the column all formats which have this id
func (fs *FormatStructure) OrderFormatsByID() ([][]*crunchyroll.Format, error) {
	formats, err := fs.Formats()
	if err != nil {
		return nil, err
	}

	formatsMap := map[string][]*crunchyroll.Format{}
	for _, format := range formats {
		if _, ok := formatsMap[format.ID]; !ok {
			formatsMap[format.ID] = make([]*crunchyroll.Format, 0)
		}
		formatsMap[format.ID] = append(formatsMap[format.ID], format)
	}

	var orderedFormats [][]*crunchyroll.Format
	for _, v := range formatsMap {
		var f []*crunchyroll.Format
		for _, format := range v {
			f = append(f, format)
		}
		orderedFormats = append(orderedFormats, f)
	}
	return orderedFormats, nil
}

// StreamStructure fields are nearly same as FormatStructure
type StreamStructure struct {
	*FormatStructure

	getFunc func() ([]*crunchyroll.Stream, []crunchyroll.Video, error)
	streams []*crunchyroll.Stream
	parents []crunchyroll.Video
}

func newStreamStructure(structure VideoStructure) *StreamStructure {
	var getFunc func() (streams []*crunchyroll.Stream, parents []crunchyroll.Video, err error)
	switch structure.(type) {
	case *EpisodeStructure:
		episodeStructure := structure.(*EpisodeStructure)
		getFunc = func() (streams []*crunchyroll.Stream, parents []crunchyroll.Video, err error) {
			episodes, err := episodeStructure.Episodes()
			if err != nil {
				return
			}

			var wg sync.WaitGroup
			var lock sync.Mutex

			for _, episode := range episodes {
				wg.Add(1)
				episode := episode
				go func() {
					defer wg.Done()
					s, err := episode.Streams()
					if err != nil {
						return
					}
					lock.Lock()
					defer lock.Unlock()
					for _, stream := range s {
						streams = append(streams, stream)
						parents = append(parents, episode)
					}
				}()
			}
			wg.Wait()
			return
		}
	case *MovieListingStructure:
		movieListingStructure := structure.(*MovieListingStructure)
		getFunc = func() (streams []*crunchyroll.Stream, parents []crunchyroll.Video, err error) {
			movieListings, err := movieListingStructure.MovieListings()
			if err != nil {
				return
			}

			var wg sync.WaitGroup
			var lock sync.Mutex

			for _, movieListing := range movieListings {
				wg.Add(1)
				movieListing := movieListing
				go func() {
					defer wg.Done()
					s, err := movieListing.Streams()
					if err != nil {
						return
					}
					lock.Lock()
					defer lock.Unlock()
					for _, stream := range s {
						streams = append(streams, stream)
						parents = append(parents, movieListing)
					}
				}()
			}
			wg.Wait()
			return
		}
	}

	ss := &StreamStructure{
		getFunc: getFunc,
	}
	ss.FormatStructure = newFormatStructure(ss)
	return ss
}

// NewStreamStructure returns a new StreamStructure, based on the given formats
func NewStreamStructure(streams []*crunchyroll.Stream) *StreamStructure {
	ss := &StreamStructure{
		getFunc: func() ([]*crunchyroll.Stream, []crunchyroll.Video, error) {
			return streams, nil, nil
		},
	}
	ss.FormatStructure = newFormatStructure(ss)
	return ss
}

// Streams returns all stored streams
func (ss *StreamStructure) Streams() ([]*crunchyroll.Stream, error) {
	if ss.streams == nil {
		var err error
		if ss.streams, ss.parents, err = ss.getFunc(); err != nil {
			return nil, err
		}
	}
	return ss.streams, nil
}

// StreamParent returns the parent video (type crunchyroll.Series or crunchyroll.Movie) of a stream (if present).
// If the stream or parent is not stored, an error will be returned
func (ss *StreamStructure) StreamParent(stream *crunchyroll.Stream) (crunchyroll.Video, error) {
	streams, err := ss.Streams()
	if err != nil {
		return nil, err
	}

	if ss.parents == nil {
		return nil, errors.New("no parents are given")
	}

	for i, s := range streams {
		if s == stream {
			return ss.parents[i], nil
		}
	}
	return nil, errors.New("given stream could not be found")
}

// VideoStructure is an interface which is implemented by EpisodeStructure and MovieListingStructure
type VideoStructure interface{}

// EpisodeStructure fields are nearly same as FormatStructure
type EpisodeStructure struct {
	VideoStructure
	*StreamStructure

	getFunc  func() ([]*crunchyroll.Episode, []*crunchyroll.Season, error)
	episodes []*crunchyroll.Episode
	parents  []*crunchyroll.Season
}

func newEpisodeStructure(structure *SeasonStructure) *EpisodeStructure {
	es := &EpisodeStructure{
		getFunc: func() (episodes []*crunchyroll.Episode, parents []*crunchyroll.Season, err error) {
			seasons, err := structure.Seasons()
			if err != nil {
				return
			}

			var wg sync.WaitGroup
			var lock sync.Mutex

			for _, season := range seasons {
				wg.Add(1)
				season := season
				go func() {
					defer wg.Done()
					e, err := season.Episodes()
					if err != nil {
						return
					}
					lock.Lock()
					defer lock.Unlock()
					for _, episode := range e {
						episodes = append(episodes, episode)
						parents = append(parents, season)
					}
				}()
			}
			wg.Wait()
			return
		},
	}
	es.StreamStructure = newStreamStructure(es)
	return es
}

// NewEpisodeStructure returns a new EpisodeStructure, based on the given formats
func NewEpisodeStructure(episodes []*crunchyroll.Episode) *EpisodeStructure {
	es := &EpisodeStructure{
		getFunc: func() ([]*crunchyroll.Episode, []*crunchyroll.Season, error) {
			return episodes, nil, nil
		},
	}
	es.StreamStructure = newStreamStructure(es)
	return es
}

// Episodes returns all stored episodes
func (es *EpisodeStructure) Episodes() ([]*crunchyroll.Episode, error) {
	if es.episodes == nil {
		var err error
		if es.episodes, es.parents, err = es.getFunc(); err != nil {
			return nil, err
		}
	}
	return es.episodes, nil
}

// EpisodeParent returns the parent season of a stream (if present).
// If the stream or parent is not stored, an error will be returned
func (es *EpisodeStructure) EpisodeParent(episode *crunchyroll.Episode) (*crunchyroll.Season, error) {
	episodes, err := es.Episodes()
	if err != nil {
		return nil, err
	}

	if es.parents == nil {
		return nil, errors.New("no parents are given")
	}

	for i, e := range episodes {
		if e == episode {
			return es.parents[i], nil
		}
	}
	return nil, errors.New("given episode could not be found")
}

// GetEpisodeByFormat returns the episode to which the given format belongs to.
// If the format or the parent is not stored, an error will be returned
func (es *EpisodeStructure) GetEpisodeByFormat(format *crunchyroll.Format) (*crunchyroll.Episode, error) {
	if !es.initState {
		if err := es.InitAll(); err != nil {
			return nil, err
		}
	}

	formatParent, err := es.FormatParent(format)
	if err != nil {
		return nil, err
	}
	streamParent, err := es.StreamParent(formatParent)
	if err != nil {
		return nil, err
	}
	episode, ok := streamParent.(*crunchyroll.Episode)
	if !ok {
		return nil, errors.New("could not find parent episode")
	}
	return episode, nil
}

func (es *EpisodeStructure) OrderEpisodeByID() ([][]*crunchyroll.Episode, error) {
	episodes, err := es.Episodes()
	if err != nil {
		return nil, err
	}

	episodesMap := map[string][]*crunchyroll.Episode{}
	for _, episode := range episodes {
		if _, ok := episodesMap[episode.ID]; !ok {
			episodesMap[episode.ID] = make([]*crunchyroll.Episode, 0)
		}
		episodesMap[episode.ID] = append(episodesMap[episode.ID], episode)
	}

	var orderedEpisodes [][]*crunchyroll.Episode
	for _, v := range episodesMap {
		orderedEpisodes = append(orderedEpisodes, v)
	}
	return orderedEpisodes, nil
}

func (es *EpisodeStructure) OrderFormatsByEpisodeNumber() ([][]*crunchyroll.Format, error) {
	formats, err := es.Formats()
	if err != nil {
		return nil, err
	}

	formatsMap := map[int][]*crunchyroll.Format{}
	for _, format := range formats {
		stream, err := es.FormatParent(format)
		if err != nil {
			return nil, err
		}
		video, err := es.StreamParent(stream)
		if err != nil {
			return nil, err
		}

		episode, ok := video.(*crunchyroll.Episode)
		if !ok {
			continue
		}
		if _, ok := formatsMap[episode.EpisodeNumber]; !ok {
			formatsMap[episode.EpisodeNumber] = make([]*crunchyroll.Format, 0)
		}
		formatsMap[episode.EpisodeNumber] = append(formatsMap[episode.EpisodeNumber], format)
	}

	keys := make([]int, 0, len(formatsMap))
	for k := range formatsMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var orderedFormats [][]*crunchyroll.Format
	for _, k := range keys {
		orderedFormats = append(orderedFormats, formatsMap[k])
	}
	return orderedFormats, nil
}

// SeasonStructure fields are nearly same as FormatStructure
type SeasonStructure struct {
	*EpisodeStructure

	getFunc func() ([]*crunchyroll.Season, error)
	seasons []*crunchyroll.Season
}

// NewSeasonStructure returns a new SeasonStructure, based on the given formats
func NewSeasonStructure(seasons []*crunchyroll.Season) *SeasonStructure {
	ss := &SeasonStructure{
		seasons: seasons,
	}
	ss.EpisodeStructure = newEpisodeStructure(ss)
	return ss
}

// Seasons returns all stored seasons
func (ss *SeasonStructure) Seasons() ([]*crunchyroll.Season, error) {
	if ss.seasons == nil {
		var err error
		if ss.seasons, err = ss.getFunc(); err != nil {
			return nil, err
		}
	}
	return ss.seasons, nil
}

// MovieListingStructure fields are nearly same as FormatStructure
type MovieListingStructure struct {
	VideoStructure
	*StreamStructure

	getFunc       func() ([]*crunchyroll.MovieListing, error)
	movieListings []*crunchyroll.MovieListing
}

// NewMovieListingStructure returns a new MovieListingStructure, based on the given formats
func NewMovieListingStructure(movieListings []*crunchyroll.MovieListing) *MovieListingStructure {
	ml := &MovieListingStructure{
		getFunc: func() ([]*crunchyroll.MovieListing, error) {
			return movieListings, nil
		},
	}
	ml.StreamStructure = newStreamStructure(ml)
	return ml
}

// MovieListings returns all stored movie listings
func (ml *MovieListingStructure) MovieListings() ([]*crunchyroll.MovieListing, error) {
	if ml.movieListings == nil {
		var err error
		if ml.movieListings, err = ml.getFunc(); err != nil {
			return nil, err
		}
	}
	return ml.movieListings, nil
}

// GetMovieListingByFormat returns the movie listing to which the given format belongs to.
// If the format or the parent is not stored, an error will be returned
func (ml *MovieListingStructure) GetMovieListingByFormat(format *crunchyroll.Format) (*crunchyroll.MovieListing, error) {
	if !ml.initState {
		if err := ml.InitAll(); err != nil {
			return nil, err
		}
	}

	formatParent, err := ml.FormatParent(format)
	if err != nil {
		return nil, err
	}
	streamParent, err := ml.StreamParent(formatParent)
	if err != nil {
		return nil, err
	}
	movieListing, ok := streamParent.(*crunchyroll.MovieListing)
	if !ok {
		return nil, errors.New("could not find parent movie listing")
	}
	return movieListing, nil
}
