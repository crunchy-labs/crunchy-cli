use crate::download::Download;
use crate::utils::filter::Filter;
use crate::utils::format::{Format, SingleFormat, SingleFormatCollection};
use crate::utils::interactive_select::{check_for_duplicated_seasons, get_duplicated_seasons};
use crate::utils::parse::UrlFilter;
use anyhow::{bail, Result};
use crunchyroll_rs::{Concert, Episode, Movie, MovieListing, MusicVideo, Season, Series};
use log::{error, info, warn};
use std::collections::HashMap;

pub(crate) struct DownloadFilter {
    url_filter: UrlFilter,
    download: Download,
    interactive_input: bool,
    season_episode_count: HashMap<u32, Vec<String>>,
    season_subtitles_missing: Vec<u32>,
}

impl DownloadFilter {
    pub(crate) fn new(url_filter: UrlFilter, download: Download, interactive_input: bool) -> Self {
        Self {
            url_filter,
            download,
            interactive_input,
            season_episode_count: HashMap::new(),
            season_subtitles_missing: vec![],
        }
    }
}

#[async_trait::async_trait]
impl Filter for DownloadFilter {
    type T = SingleFormat;
    type Output = SingleFormatCollection;

    async fn visit_series(&mut self, series: Series) -> Result<Vec<Season>> {
        // `series.audio_locales` isn't always populated b/c of crunchyrolls api. so check if the
        // audio is matching only if the field is populated
        if !series.audio_locales.is_empty() {
            if !series.audio_locales.contains(&self.download.audio) {
                error!(
                    "Series {} is not available with {} audio",
                    series.title, self.download.audio
                );
                return Ok(vec![]);
            }
        }

        let mut seasons = vec![];
        for mut season in series.seasons().await? {
            if !self.url_filter.is_season_valid(season.season_number) {
                continue;
            }

            if !season
                .audio_locales
                .iter()
                .any(|l| l == &self.download.audio)
            {
                if season
                    .available_versions()
                    .await?
                    .iter()
                    .any(|l| l == &self.download.audio)
                {
                    season = season
                        .version(vec![self.download.audio.clone()])
                        .await?
                        .remove(0)
                } else {
                    error!(
                        "Season {} - '{}' is not available with {} audio",
                        season.season_number,
                        season.title,
                        self.download.audio.clone(),
                    );
                    continue;
                }
            }

            seasons.push(season)
        }

        let duplicated_seasons = get_duplicated_seasons(&seasons);
        if duplicated_seasons.len() > 0 {
            if self.interactive_input {
                check_for_duplicated_seasons(&mut seasons);
            } else {
                info!(
                    "Found duplicated seasons: {}",
                    duplicated_seasons
                        .iter()
                        .map(|d| d.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }
        }

        Ok(seasons)
    }

    async fn visit_season(&mut self, season: Season) -> Result<Vec<Episode>> {
        let mut episodes = season.episodes().await?;

        if Format::has_relative_episodes_fmt(&self.download.output) {
            for episode in episodes.iter() {
                self.season_episode_count
                    .entry(episode.season_number)
                    .or_insert(vec![])
                    .push(episode.id.clone())
            }
        }

        episodes.retain(|e| {
            self.url_filter
                .is_episode_valid(e.episode_number, season.season_number)
        });

        Ok(episodes)
    }

    async fn visit_episode(&mut self, mut episode: Episode) -> Result<Option<Self::T>> {
        if !self
            .url_filter
            .is_episode_valid(episode.episode_number, episode.season_number)
        {
            return Ok(None);
        }

        // check if the audio locale is correct.
        // should only be incorrect if the console input was a episode url. otherwise
        // `DownloadFilter::visit_season` returns the correct episodes with matching audio
        if episode.audio_locale != self.download.audio {
            // check if any other version (same episode, other language) of this episode is available
            // with the requested audio. if not, return an error
            if !episode
                .available_versions()
                .await?
                .contains(&self.download.audio)
            {
                bail!(
                    "Episode {} ({}) of {} season {} is not available with {} audio",
                    episode.episode_number,
                    episode.title,
                    episode.series_title,
                    episode.season_number,
                    self.download.audio
                )
            }
            // overwrite the current episode with the other version episode
            episode = episode
                .version(vec![self.download.audio.clone()])
                .await?
                .remove(0)
        }

        // check if the subtitles are supported
        if let Some(subtitle_locale) = &self.download.subtitle {
            if !episode.subtitle_locales.contains(subtitle_locale) {
                // if the episode doesn't have the requested subtitles, print a error. to print this
                // error only once per season, it's checked if an error got printed before by looking
                // up if the season id is present in `self.season_subtitles_missing`. if not, print
                // the error and add the season id to `self.season_subtitles_missing`. if it is
                // present, skip the error printing
                if !self
                    .season_subtitles_missing
                    .contains(&episode.season_number)
                {
                    self.season_subtitles_missing.push(episode.season_number);
                    error!(
                        "{} season {} is not available with {} subtitles",
                        episode.series_title, episode.season_number, subtitle_locale
                    );
                }
                return Ok(None);
            }
        }

        // get the relative episode number. only done if the output string has the pattern to include
        // the relative episode number as this requires some extra fetching
        let relative_episode_number = if Format::has_relative_episodes_fmt(&self.download.output) {
            if self
                .season_episode_count
                .get(&episode.season_number)
                .is_none()
            {
                let season_episodes = episode.season().await?.episodes().await?;
                self.season_episode_count.insert(
                    episode.season_number,
                    season_episodes.into_iter().map(|e| e.id).collect(),
                );
            }
            let relative_episode_number = self
                .season_episode_count
                .get(&episode.season_number)
                .unwrap()
                .iter()
                .position(|id| id == &episode.id)
                .map(|index| index + 1);
            if relative_episode_number.is_none() {
                warn!(
                    "Failed to get relative episode number for episode {} ({}) of {} season {}",
                    episode.episode_number,
                    episode.title,
                    episode.series_title,
                    episode.season_number,
                )
            }
            relative_episode_number
        } else {
            None
        };

        Ok(Some(SingleFormat::new_from_episode(
            episode.clone(),
            self.download.subtitle.clone().map_or(vec![], |s| {
                if episode.subtitle_locales.contains(&s) {
                    vec![s]
                } else {
                    vec![]
                }
            }),
            relative_episode_number.map(|n| n as u32),
        )))
    }

    async fn visit_movie_listing(&mut self, movie_listing: MovieListing) -> Result<Vec<Movie>> {
        Ok(movie_listing.movies().await?)
    }

    async fn visit_movie(&mut self, movie: Movie) -> Result<Option<Self::T>> {
        Ok(Some(SingleFormat::new_from_movie(movie, vec![])))
    }

    async fn visit_music_video(&mut self, music_video: MusicVideo) -> Result<Option<Self::T>> {
        Ok(Some(SingleFormat::new_from_music_video(music_video)))
    }

    async fn visit_concert(&mut self, concert: Concert) -> Result<Option<Self::T>> {
        Ok(Some(SingleFormat::new_from_concert(concert)))
    }

    async fn finish(self, input: Vec<Self::T>) -> Result<Self::Output> {
        let mut single_format_collection = SingleFormatCollection::new();

        for data in input {
            single_format_collection.add_single_formats(vec![data])
        }

        Ok(single_format_collection)
    }
}
