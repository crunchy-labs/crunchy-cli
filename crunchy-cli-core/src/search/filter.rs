use crate::utils::parse::UrlFilter;
use crunchyroll_rs::media::Subtitle;
use crunchyroll_rs::{Episode, Locale, MovieListing, Season, Series};

pub struct FilterOptions {
    pub audio: Vec<Locale>,
    pub filter_subtitles: bool,
    pub url_filter: UrlFilter,
}

impl FilterOptions {
    pub fn check_series(&self, series: &Series) -> bool {
        self.check_audio_language(&series.audio_locales)
    }

    pub fn filter_seasons(&self, mut seasons: Vec<Season>) -> Vec<Season> {
        seasons.retain(|s| {
            self.check_audio_language(&s.audio_locales)
                && self.url_filter.is_season_valid(s.season_number)
        });
        seasons
    }

    pub fn filter_episodes(&self, mut episodes: Vec<Episode>) -> Vec<Episode> {
        episodes.retain(|e| {
            self.check_audio_language(&vec![e.audio_locale.clone()])
                && self
                    .url_filter
                    .is_episode_valid(e.episode_number, e.season_number)
        });
        episodes
    }

    pub fn check_movie_listing(&self, movie_listing: &MovieListing) -> bool {
        self.check_audio_language(
            &movie_listing
                .audio_locale
                .clone()
                .map_or(vec![], |a| vec![a.clone()]),
        )
    }

    pub fn filter_subtitles(&self, mut subtitles: Vec<Subtitle>) -> Vec<Subtitle> {
        if self.filter_subtitles {
            subtitles.retain(|s| self.check_audio_language(&vec![s.locale.clone()]))
        }
        subtitles
    }

    fn check_audio_language(&self, audio: &Vec<Locale>) -> bool {
        if !self.audio.is_empty() {
            return self.audio.iter().any(|a| audio.contains(a));
        }
        true
    }
}
