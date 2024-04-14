use crate::archive::command::Archive;
use crate::utils::filter::{real_dedup_vec, Filter};
use crate::utils::format::{Format, SingleFormat, SingleFormatCollection};
use crate::utils::interactive_select::{check_for_duplicated_seasons, get_duplicated_seasons};
use crate::utils::parse::{fract, UrlFilter};
use anyhow::Result;
use crunchyroll_rs::{Concert, Episode, Locale, Movie, MovieListing, MusicVideo, Season, Series};
use log::{info, warn};
use std::collections::{BTreeMap, HashMap};
use std::ops::Not;

enum Visited {
    Series,
    Season,
    None,
}

pub(crate) struct ArchiveFilter {
    url_filter: UrlFilter,
    archive: Archive,
    interactive_input: bool,
    skip_special: bool,
    season_episodes: HashMap<String, Vec<Episode>>,
    season_subtitles_missing: Vec<u32>,
    seasons_with_premium: Option<Vec<u32>>,
    season_sorting: Vec<String>,
    visited: Visited,
}

impl ArchiveFilter {
    pub(crate) fn new(
        url_filter: UrlFilter,
        archive: Archive,
        interactive_input: bool,
        skip_special: bool,
        is_premium: bool,
    ) -> Self {
        Self {
            url_filter,
            archive,
            interactive_input,
            skip_special,
            season_episodes: HashMap::new(),
            season_subtitles_missing: vec![],
            seasons_with_premium: is_premium.not().then_some(vec![]),
            season_sorting: vec![],
            visited: Visited::None,
        }
    }
}

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
        if !duplicated_seasons.is_empty() {
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
                .flat_map(|s| s.audio_locales.clone())
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
                .flat_map(|s| s.subtitle_locales.clone())
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
                        .first()
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
                if eps.is_empty() {
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
                        last_episode.sequence_number,
                        last_episode.title
                    )
                }
            }
            episodes.extend(eps)
        }

        if Format::has_relative_fmt(&self.archive.output) {
            for episode in episodes.iter() {
                self.season_episodes
                    .entry(episode.season_id.clone())
                    .or_default()
                    .push(episode.clone())
            }
        }

        Ok(episodes)
    }

    async fn visit_episode(&mut self, mut episode: Episode) -> Result<Option<Self::T>> {
        if !self
            .url_filter
            .is_episode_valid(episode.sequence_number, episode.season_number)
        {
            return Ok(None);
        }

        // skip the episode if it's a special
        if self.skip_special
            && (episode.sequence_number == 0.0 || episode.sequence_number.fract() != 0.0)
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
                    episode.sequence_number,
                    missing_audio
                        .into_iter()
                        .map(|l| l.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                )
            }

            let mut subtitle_locales: Vec<Locale> =
                episodes.iter().flat_map(|(_, s)| s.clone()).collect();
            real_dedup_vec(&mut subtitle_locales);
            let missing_subtitles = missing_locales(&subtitle_locales, &self.archive.subtitle);
            if !missing_subtitles.is_empty()
                && !self
                    .season_subtitles_missing
                    .contains(&episode.season_number)
            {
                warn!(
                    "Episode {} is not available with {} subtitles",
                    episode.sequence_number,
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

        if self.seasons_with_premium.is_some() {
            let episode_len_before = episodes.len();
            episodes.retain(|(e, _)| !e.is_premium_only);
            if episode_len_before < episodes.len()
                && !self
                    .seasons_with_premium
                    .as_ref()
                    .unwrap()
                    .contains(&episode.season_number)
            {
                warn!(
                    "Skipping premium episodes in season {}",
                    episode.season_number
                );
                self.seasons_with_premium
                    .as_mut()
                    .unwrap()
                    .push(episode.season_number)
            }

            return Ok(None);
        }

        let mut relative_episode_number = None;
        let mut relative_sequence_number = None;
        // get the relative episode number. only done if the output string has the pattern to include
        // the relative episode number as this requires some extra fetching
        if Format::has_relative_fmt(&self.archive.output) {
            let season_eps = match self.season_episodes.get(&episode.season_id) {
                Some(eps) => eps,
                None => {
                    self.season_episodes.insert(
                        episode.season_id.clone(),
                        episode.season().await?.episodes().await?,
                    );
                    self.season_episodes.get(&episode.season_id).unwrap()
                }
            };
            let mut non_integer_sequence_number_count = 0;
            for (i, ep) in season_eps.iter().enumerate() {
                if ep.sequence_number.fract() != 0.0 || ep.sequence_number == 0.0 {
                    non_integer_sequence_number_count += 1;
                }
                if ep.id == episode.id {
                    relative_episode_number = Some(i + 1);
                    relative_sequence_number = Some(
                        (i + 1 - non_integer_sequence_number_count) as f32
                            + fract(ep.sequence_number),
                    );
                    break;
                }
            }
            if relative_episode_number.is_none() || relative_sequence_number.is_none() {
                warn!(
                    "Failed to get relative episode number for episode {} ({}) of {} season {}",
                    episode.sequence_number,
                    episode.title,
                    episode.series_title,
                    episode.season_number,
                )
            }
        }

        Ok(Some(
            episodes
                .into_iter()
                .map(|(e, s)| {
                    SingleFormat::new_from_episode(
                        e,
                        s,
                        relative_episode_number.map(|n| n as u32),
                        relative_sequence_number,
                    )
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

fn missing_locales<'a>(available: &[Locale], searched: &'a [Locale]) -> Vec<&'a Locale> {
    searched.iter().filter(|p| !available.contains(p)).collect()
}
