package crunchyroll

import (
	"github.com/grafov/m3u8"
	"net/http"
	"os"
	"testing"
)

var (
	email     = os.Getenv("EMAIL")
	password  = os.Getenv("PASSWORD")
	sessionID = os.Getenv("SESSION_ID")

	crunchy *Crunchyroll
	season  *Season
	episode *Episode
	stream  *Stream
)

func TestLogin(t *testing.T) {
	var err error
	if email != "" && password != "" {
		crunchy, err = LoginWithCredentials(email, password, DE, http.DefaultClient)
		if err != nil {
			t.Error(err)
		}
		t.Logf("Logged in with email and password\nAuth: %s %s\nSession id: %s",
			crunchy.Config.TokenType, crunchy.Config.AccessToken, crunchy.SessionID)
	} else if sessionID != "" {
		crunchy, err = LoginWithSessionID(sessionID, DE, http.DefaultClient)
		if err != nil {
			t.Error(err)
		}
		t.Logf("Logged in with session id\nAuth: %s %s\nSession id: %s",
			crunchy.Config.TokenType, crunchy.Config.AccessToken, crunchy.SessionID)
	} else {
		t.Skipf("email and / or password and session id environtment variables are not set, skipping login. All following test may fail also")
	}
}

func TestCrunchy_Search(t *testing.T) {
	series, movies, err := crunchy.Search("movie", 20)
	if err != nil {
		t.Error(err)
	}
	t.Logf("Found %d series and %d movie(s) for search query `movie`", len(series), len(movies))
}

func TestSeries_Seasons(t *testing.T) {
	video, err := crunchy.FindVideo("https://www.crunchyroll.com/darling-in-the-franxx")
	if err != nil {
		t.Error(err)
	}
	series := video.(*Series)
	seasons, err := series.Seasons()
	if err != nil {
		t.Error(err)
	}
	if len(seasons) > 0 {
		season = seasons[4]
	} else {
		t.Logf("%s has no seasons, some future test will fail", series.Title)
	}
	t.Logf("Found %d seasons for series %s", len(seasons), series.Title)
}

func TestCrunchyroll_FindEpisode(t *testing.T) {
	episodes, err := crunchy.FindEpisode("https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575")
	if err != nil {
		t.Error(err)
	}
	t.Logf("Found %d episodes for episode %s", len(episodes), "https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575")
}

func TestSeason_Episodes(t *testing.T) {
	episodes, err := season.Episodes()
	if err != nil {
		t.Error(err)
	}
	if len(episodes) > 0 {
		episode = episodes[0]
	} else {
		t.Logf("%s has no episodes, some future test will fail", season.Title)
	}
	t.Logf("Found %d episodes for season %s", len(episodes), season.Title)
}

func TestEpisode_Streams(t *testing.T) {
	streams, err := episode.Streams()
	if err != nil {
		t.Error(err)
	}
	if len(streams) > 0 {
		stream = streams[0]
	} else {
		t.Logf("%s has no streams, some future test will fail", season.Title)
	}
	t.Logf("Found %d streams for episode %s", len(streams), season.Title)
}

func TestFormat_Download(t *testing.T) {
	formats, err := stream.Formats()
	if err != nil {
		t.Error(err)
	}
	file, err := os.Create("test")
	if err != nil {
		t.Error(err)
	}
	formats[0].DownloadGoroutines(file, 4, func(segment *m3u8.MediaSegment, current, total int, file *os.File) error {
		t.Logf("Downloaded %.2f%% (%d/%d)", float32(current)/float32(total)*100, current, total)
		return nil
	})
}
