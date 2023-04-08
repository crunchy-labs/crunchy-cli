use crate::archive::command::Archive;
use crate::utils::filter::{real_dedup_vec, Filter};
use crate::utils::format::{Format, SingleFormat, SingleFormatCollection};
use crate::utils::parse::UrlFilter;
use anyhow::Result;
use crunchyroll_rs::{Concert, Episode, Locale, Movie, MovieListing, MusicVideo, Season, Series};
use log::warn;
use std::collections::{BTreeMap, HashMap};

enum Visited {
    Series,
    Season,
    None,
}

pub(crate) struct ArchiveFilter {
    url_filter: UrlFilter,
    archive: Archive,
    season_episode_count: HashMap<u32, Vec<String>>,
    season_subtitles_missing: Vec<u32>,
    visited: Visited,
}

impl ArchiveFilter {
    pub(crate) fn new(url_filter: UrlFilter, archive: Archive) -> Self {
        Self {
            url_filter,
            archive,
            season_episode_count: HashMap::new(),
            season_subtitles_missing: vec![],
            visited: Visited::None,
        }
    }
}

#[async_trait::async_trait]
impl Filter for ArchiveFilter {
    type T = Vec<SingleFormat>;
    type Output = SingleFormatCollection;

    async fn visit_series(&mut self, series: Series) -> Result<Vec<Season>> {
        // `series.audio_locales` isn't always populated b/c of crunchyrolls api. so check if the
        // audio is matching only if the field is populated
        if !series.audio_locales.is_empty() {
            let missing_audio = missing_locales(&series.audio_locales, &self.archive.locale);
            if !missing_audio.is_empty() {
                warn!(
                    "Series {} is not available with {} audio",
                    series.title,
                    missing_audio
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }
            let missing_subtitle =
                missing_locales(&series.subtitle_locales, &self.archive.subtitle);
            if !missing_subtitle.is_empty() {
                warn!(
                    "Series {} is not available with {} subtitles",
                    series.title,
                    missing_subtitle
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }
            self.visited = Visited::Series
        }
        Ok(series.seasons().await?)
    }

    async fn visit_season(&mut self, mut season: Season) -> Result<Vec<Episode>> {
        if !self.url_filter.is_season_valid(season.season_number) {
            return Ok(vec![]);
        }

        let mut seasons = season.version(self.archive.locale.clone()).await?;
        if self
            .archive
            .locale
            .iter()
            .any(|l| season.audio_locales.contains(l))
        {
            seasons.insert(0, season.clone());
        }

        if !matches!(self.visited, Visited::Series) {
            let mut audio_locales: Vec<Locale> = seasons
                .iter()
                .map(|s| s.audio_locales.clone())
                .flatten()
                .collect();
            real_dedup_vec(&mut audio_locales);
            let missing_audio = missing_locales(&audio_locales, &self.archive.locale);
            if !missing_audio.is_empty() {
                warn!(
                    "Season {} is not available with {} audio",
                    season.season_number,
                    missing_audio
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }

            let subtitle_locales: Vec<Locale> = seasons
                .iter()
                .map(|s| s.subtitle_locales.clone())
                .flatten()
                .collect();
            let missing_subtitle = missing_locales(&subtitle_locales, &self.archive.subtitle);
            if !missing_subtitle.is_empty() {
                warn!(
                    "Season {} is not available with {} subtitles",
                    season.season_number,
                    missing_subtitle
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }
            self.visited = Visited::Season
        }

        let mut episodes = vec![];
        for season in seasons {
            episodes.extend(season.episodes().await?)
        }

        if Format::has_relative_episodes_fmt(&self.archive.output) {
            for episode in episodes.iter() {
                self.season_episode_count
                    .entry(episode.season_number)
                    .or_insert(vec![])
                    .push(episode.id.clone())
            }
        }

        Ok(episodes)
    }

    async fn visit_episode(&mut self, mut episode: Episode) -> Result<Option<Self::T>> {
        if !self
            .url_filter
            .is_episode_valid(episode.episode_number, episode.season_number)
        {
            return Ok(None);
        }

        let mut episodes = vec![];
        if !matches!(self.visited, Visited::Series) && !matches!(self.visited, Visited::Season) {
            if self.archive.locale.contains(&episode.audio_locale) {
                episodes.push((episode.clone(), episode.subtitle_locales.clone()))
            }
            episodes.extend(
                episode
                    .version(self.archive.locale.clone())
                    .await?
                    .into_iter()
                    .map(|e| (e.clone(), e.subtitle_locales.clone())),
            );
            let audio_locales: Vec<Locale> = episodes
                .iter()
                .map(|(e, _)| e.audio_locale.clone())
                .collect();
            let missing_audio = missing_locales(&audio_locales, &self.archive.locale);
            if !missing_audio.is_empty() {
                warn!(
                    "Episode {} is not available with {} audio",
                    episode.episode_number,
                    missing_audio
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }

            let mut subtitle_locales: Vec<Locale> =
                episodes.iter().map(|(_, s)| s.clone()).flatten().collect();
            real_dedup_vec(&mut subtitle_locales);
            let missing_subtitles = missing_locales(&subtitle_locales, &self.archive.subtitle);
            if !missing_subtitles.is_empty()
                && !self
                    .season_subtitles_missing
                    .contains(&episode.season_number)
            {
                warn!(
                    "Episode {} is not available with {} subtitles",
                    episode.episode_number,
                    missing_subtitles
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                );
                self.season_subtitles_missing.push(episode.season_number)
            }
        } else {
            episodes.push((episode.clone(), episode.subtitle_locales.clone()))
        }

        let relative_episode_number = if Format::has_relative_episodes_fmt(&self.archive.output) {
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

        Ok(Some(
            episodes
                .into_iter()
                .map(|(e, s)| {
                    SingleFormat::new_from_episode(e, s, relative_episode_number.map(|n| n as u32))
                })
                .collect(),
        ))
    }

    async fn visit_movie_listing(&mut self, movie_listing: MovieListing) -> Result<Vec<Movie>> {
        Ok(movie_listing.movies().await?)
    }

    async fn visit_movie(&mut self, movie: Movie) -> Result<Option<Self::T>> {
        Ok(Some(vec![SingleFormat::new_from_movie(movie, vec![])]))
    }

    async fn visit_music_video(&mut self, music_video: MusicVideo) -> Result<Option<Self::T>> {
        Ok(Some(vec![SingleFormat::new_from_music_video(music_video)]))
    }

    async fn visit_concert(&mut self, concert: Concert) -> Result<Option<Self::T>> {
        Ok(Some(vec![SingleFormat::new_from_concert(concert)]))
    }

    async fn finish(self, input: Vec<Self::T>) -> Result<Self::Output> {
        let flatten_input: Self::T = input.into_iter().flatten().collect();

        let mut single_format_collection = SingleFormatCollection::new();

        struct SortKey(u32, String);

        let mut sorted: BTreeMap<(u32, String), Self::T> = BTreeMap::new();
        for data in flatten_input {
            sorted
                .entry((data.season_number, data.sequence_number.to_string()))
                .or_insert(vec![])
                .push(data)
        }

        for data in sorted.into_values() {
            single_format_collection.add_single_formats(data)
        }

        Ok(single_format_collection)
    }
}

fn missing_locales<'a>(available: &Vec<Locale>, searched: &'a Vec<Locale>) -> Vec<&'a Locale> {
    searched.iter().filter(|p| !available.contains(p)).collect()
}
