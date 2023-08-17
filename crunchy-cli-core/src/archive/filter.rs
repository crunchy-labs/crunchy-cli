use crate::archive::command::Archive;
use crate::utils::filter::{real_dedup_vec, Filter};
use crate::utils::format::{Format, SingleFormat, SingleFormatCollection};
use crate::utils::interactive_select::{check_for_duplicated_seasons, get_duplicated_seasons};
use crate::utils::parse::UrlFilter;
use anyhow::Result;
use crunchyroll_rs::{Concert, Episode, Locale, Movie, MovieListing, MusicVideo, Season, Series};
use log::{info, warn};
use std::collections::{BTreeMap, HashMap};

enum Visited {
    Series,
    Season,
    None,
}

pub(crate) struct ArchiveFilter {
    url_filter: UrlFilter,
    archive: Archive,
    interactive_input: bool,
    season_episode_count: HashMap<String, Vec<String>>,
    season_subtitles_missing: Vec<u32>,
    season_sorting: Vec<String>,
    visited: Visited,
}

impl ArchiveFilter {
    pub(crate) fn new(url_filter: UrlFilter, archive: Archive, interactive_input: bool) -> Self {
        Self {
            url_filter,
            archive,
            interactive_input,
            season_episode_count: HashMap::new(),
            season_subtitles_missing: vec![],
            season_sorting: vec![],
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
            let missing_audio = missing_locales(&series.audio_locales, &self.archive.audio);
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

        let mut seasons = series.seasons().await?;
        let mut remove_ids = vec![];
        for season in seasons.iter_mut() {
            if !self.url_filter.is_season_valid(season.season_number)
                || (!season
                    .audio_locales
                    .iter()
                    .any(|l| self.archive.audio.contains(l))
                    && !season
                        .available_versions()
                        .await?
                        .iter()
                        .any(|l| self.archive.audio.contains(l)))
            {
                remove_ids.push(season.id.clone());
            }
        }

        seasons.retain(|s| !remove_ids.contains(&s.id));

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

    async fn visit_season(&mut self, mut season: Season) -> Result<Vec<Episode>> {
        if !self.url_filter.is_season_valid(season.season_number) {
            return Ok(vec![]);
        }

        let mut seasons = season.version(self.archive.audio.clone()).await?;
        if self
            .archive
            .audio
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
            let missing_audio = missing_locales(&audio_locales, &self.archive.audio);
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
            self.season_sorting.push(season.id.clone());
            let season_locale = if season.audio_locales.len() < 2 {
                Some(
                    season
                        .audio_locales
                        .get(0)
                        .cloned()
                        .unwrap_or(Locale::ja_JP),
                )
            } else {
                None
            };
            let mut eps = season.episodes().await?;
            let before_len = eps.len();

            for mut ep in eps.clone() {
                if let Some(l) = &season_locale {
                    if &ep.audio_locale == l {
                        continue;
                    }
                    eps.remove(eps.iter().position(|p| p.id == ep.id).unwrap());
                } else {
                    let mut requested_locales = self.archive.audio.clone();
                    if let Some(idx) = requested_locales.iter().position(|p| p == &ep.audio_locale)
                    {
                        requested_locales.remove(idx);
                    } else {
                        eps.remove(eps.iter().position(|p| p.id == ep.id).unwrap());
                    }
                    eps.extend(ep.version(self.archive.audio.clone()).await?);
                }
            }
            if eps.len() < before_len {
                if eps.len() == 0 {
                    if matches!(self.visited, Visited::Series) {
                        warn!(
                            "Season {} is not available with {} audio",
                            season.season_number,
                            season_locale.unwrap_or(Locale::ja_JP)
                        )
                    }
                } else {
                    let last_episode = eps.last().unwrap();
                    warn!(
                        "Season {} is only available with {} audio until episode {} ({})",
                        season.season_number,
                        season_locale.unwrap_or(Locale::ja_JP),
                        last_episode.episode_number,
                        last_episode.title
                    )
                }
            }
            episodes.extend(eps)
        }

        if Format::has_relative_episodes_fmt(&self.archive.output) {
            for episode in episodes.iter() {
                self.season_episode_count
                    .entry(episode.season_id.clone())
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
            if self.archive.audio.contains(&episode.audio_locale) {
                episodes.push((episode.clone(), episode.subtitle_locales.clone()))
            }
            episodes.extend(
                episode
                    .version(self.archive.audio.clone())
                    .await?
                    .into_iter()
                    .map(|e| (e.clone(), e.subtitle_locales.clone())),
            );
            let audio_locales: Vec<Locale> = episodes
                .iter()
                .map(|(e, _)| e.audio_locale.clone())
                .collect();
            let missing_audio = missing_locales(&audio_locales, &self.archive.audio);
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
            if self.season_episode_count.get(&episode.season_id).is_none() {
                let season_episodes = episode.season().await?.episodes().await?;
                self.season_episode_count.insert(
                    episode.season_id.clone(),
                    season_episodes.into_iter().map(|e| e.id).collect(),
                );
            }
            let relative_episode_number = self
                .season_episode_count
                .get(&episode.season_id)
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

        let mut pre_sorted: BTreeMap<String, Self::T> = BTreeMap::new();
        for data in flatten_input {
            pre_sorted
                .entry(data.identifier.clone())
                .or_insert(vec![])
                .push(data)
        }

        let mut sorted: Vec<(String, Self::T)> = pre_sorted.into_iter().collect();
        sorted.sort_by(|(_, a), (_, b)| {
            self.season_sorting
                .iter()
                .position(|p| p == &a.first().unwrap().season_id)
                .unwrap()
                .cmp(
                    &self
                        .season_sorting
                        .iter()
                        .position(|p| p == &b.first().unwrap().season_id)
                        .unwrap(),
                )
        });

        for (_, mut data) in sorted {
            data.sort_by(|a, b| {
                self.archive
                    .audio
                    .iter()
                    .position(|p| p == &a.audio)
                    .unwrap_or(usize::MAX)
                    .cmp(
                        &self
                            .archive
                            .audio
                            .iter()
                            .position(|p| p == &b.audio)
                            .unwrap_or(usize::MAX),
                    )
            });
            single_format_collection.add_single_formats(data)
        }

        Ok(single_format_collection)
    }
}

fn missing_locales<'a>(available: &Vec<Locale>, searched: &'a Vec<Locale>) -> Vec<&'a Locale> {
    searched.iter().filter(|p| !available.contains(p)).collect()
}
