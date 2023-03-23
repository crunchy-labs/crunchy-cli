use crate::archive::command::Archive;
use crate::utils::download::{DownloadBuilder, DownloadFormat, Downloader, MergeBehavior};
use crate::utils::filter::{real_dedup_vec, Filter};
use crate::utils::format::{Format, SingleFormat};
use crate::utils::parse::UrlFilter;
use crate::utils::video::variant_data_from_stream;
use anyhow::{bail, Result};
use chrono::Duration;
use crunchyroll_rs::media::{Subtitle, VariantData};
use crunchyroll_rs::{Concert, Episode, Locale, Movie, MovieListing, MusicVideo, Season, Series};
use log::warn;
use std::collections::HashMap;
use std::hash::Hash;

pub(crate) struct FilterResult {
    format: SingleFormat,
    video: VariantData,
    audio: VariantData,
    duration: Duration,
    subtitles: Vec<Subtitle>,
}

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
    type T = Vec<FilterResult>;
    type Output = (Downloader, Format);

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
                episodes.push(episode.clone())
            }
            episodes.extend(episode.version(self.archive.locale.clone()).await?);
            let audio_locales: Vec<Locale> =
                episodes.iter().map(|e| e.audio_locale.clone()).collect();
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

            let mut subtitle_locales: Vec<Locale> = episodes
                .iter()
                .map(|e| e.subtitle_locales.clone())
                .flatten()
                .collect();
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
            episodes.push(episode.clone())
        }

        let mut formats = vec![];
        for episode in episodes {
            let stream = episode.streams().await?;
            let (video, audio) = if let Some((video, audio)) =
                variant_data_from_stream(&stream, &self.archive.resolution).await?
            {
                (video, audio)
            } else {
                bail!(
                    "Resolution ({}) is not available for episode {} ({}) of {} season {}",
                    &self.archive.resolution,
                    episode.episode_number,
                    episode.title,
                    episode.series_title,
                    episode.season_number,
                );
            };
            let subtitles: Vec<Subtitle> = self
                .archive
                .subtitle
                .iter()
                .filter_map(|s| stream.subtitles.get(s).cloned())
                .collect();

            let relative_episode_number = if Format::has_relative_episodes_fmt(&self.archive.output)
            {
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
                    .position(|id| id == &episode.id);
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

            formats.push(FilterResult {
                format: SingleFormat::new_from_episode(
                    &episode,
                    &video,
                    subtitles.iter().map(|s| s.locale.clone()).collect(),
                    relative_episode_number.map(|n| n as u32),
                ),
                video,
                audio,
                duration: episode.duration.clone(),
                subtitles,
            })
        }

        Ok(Some(formats))
    }

    async fn visit_movie_listing(&mut self, movie_listing: MovieListing) -> Result<Vec<Movie>> {
        Ok(movie_listing.movies().await?)
    }

    async fn visit_movie(&mut self, movie: Movie) -> Result<Option<Self::T>> {
        let stream = movie.streams().await?;
        let subtitles: Vec<&Subtitle> = self
            .archive
            .subtitle
            .iter()
            .filter_map(|l| stream.subtitles.get(l))
            .collect();

        let missing_subtitles = missing_locales(
            &subtitles.iter().map(|&s| s.locale.clone()).collect(),
            &self.archive.subtitle,
        );
        if !missing_subtitles.is_empty() {
            warn!(
                "Movie '{}' is not available with {} subtitles",
                movie.title,
                missing_subtitles
                    .into_iter()
                    .map(|l| l.to_string())
                    .collect::<Vec<String>>()
                    .join(", ")
            )
        }

        let (video, audio) = if let Some((video, audio)) =
            variant_data_from_stream(&stream, &self.archive.resolution).await?
        {
            (video, audio)
        } else {
            bail!(
                "Resolution ({}) of movie {} is not available",
                self.archive.resolution,
                movie.title
            )
        };

        Ok(Some(vec![FilterResult {
            format: SingleFormat::new_from_movie(&movie, &video, vec![]),
            video,
            audio,
            duration: movie.duration,
            subtitles: vec![],
        }]))
    }

    async fn visit_music_video(&mut self, music_video: MusicVideo) -> Result<Option<Self::T>> {
        let stream = music_video.streams().await?;
        let (video, audio) = if let Some((video, audio)) =
            variant_data_from_stream(&stream, &self.archive.resolution).await?
        {
            (video, audio)
        } else {
            bail!(
                "Resolution ({}) of music video {} is not available",
                self.archive.resolution,
                music_video.title
            )
        };

        Ok(Some(vec![FilterResult {
            format: SingleFormat::new_from_music_video(&music_video, &video),
            video,
            audio,
            duration: music_video.duration,
            subtitles: vec![],
        }]))
    }

    async fn visit_concert(&mut self, concert: Concert) -> Result<Option<Self::T>> {
        let stream = concert.streams().await?;
        let (video, audio) = if let Some((video, audio)) =
            variant_data_from_stream(&stream, &self.archive.resolution).await?
        {
            (video, audio)
        } else {
            bail!(
                "Resolution ({}x{}) of music video {} is not available",
                self.archive.resolution.width,
                self.archive.resolution.height,
                concert.title
            )
        };

        Ok(Some(vec![FilterResult {
            format: SingleFormat::new_from_concert(&concert, &video),
            video,
            audio,
            duration: concert.duration,
            subtitles: vec![],
        }]))
    }

    async fn finish(self, input: Vec<Self::T>) -> Result<Vec<Self::Output>> {
        let flatten_input: Vec<FilterResult> = input.into_iter().flatten().collect();

        #[derive(Hash, Eq, PartialEq)]
        struct SortKey {
            season: u32,
            episode: String,
        }

        let mut sorted: HashMap<SortKey, Vec<FilterResult>> = HashMap::new();
        for data in flatten_input {
            sorted
                .entry(SortKey {
                    season: data.format.season_number,
                    episode: data.format.episode_number.to_string(),
                })
                .or_insert(vec![])
                .push(data)
        }

        let mut values: Vec<Vec<FilterResult>> = sorted.into_values().collect();
        values.sort_by(|a, b| {
            a.first()
                .unwrap()
                .format
                .sequence_number
                .total_cmp(&b.first().unwrap().format.sequence_number)
        });

        let mut result = vec![];
        for data in values {
            let single_formats: Vec<SingleFormat> =
                data.iter().map(|fr| fr.format.clone()).collect();
            let format = Format::from_single_formats(single_formats);

            let mut downloader = DownloadBuilder::new()
                .default_subtitle(self.archive.default_subtitle.clone())
                .ffmpeg_preset(self.archive.ffmpeg_preset.clone().unwrap_or_default())
                .output_format(Some("matroska".to_string()))
                .audio_sort(Some(self.archive.locale.clone()))
                .subtitle_sort(Some(self.archive.subtitle.clone()))
                .build();

            match self.archive.merge.clone() {
                MergeBehavior::Video => {
                    for d in data {
                        downloader.add_format(DownloadFormat {
                            video: (d.video, d.format.audio.clone()),
                            audios: vec![(d.audio, d.format.audio.clone())],
                            subtitles: d.subtitles,
                        })
                    }
                }
                MergeBehavior::Audio => downloader.add_format(DownloadFormat {
                    video: (
                        data.first().unwrap().video.clone(),
                        data.first().unwrap().format.audio.clone(),
                    ),
                    audios: data
                        .iter()
                        .map(|d| (d.audio.clone(), d.format.audio.clone()))
                        .collect(),
                    // mix all subtitles together and then reduce them via a map so that only one
                    // subtitle per language exists
                    subtitles: data
                        .iter()
                        .flat_map(|d| d.subtitles.clone())
                        .map(|s| (s.locale.clone(), s))
                        .collect::<HashMap<Locale, Subtitle>>()
                        .into_values()
                        .collect(),
                }),
                MergeBehavior::Auto => {
                    let mut download_formats: HashMap<Duration, DownloadFormat> = HashMap::new();

                    for d in data {
                        if let Some(download_format) = download_formats.get_mut(&d.duration) {
                            download_format.audios.push((d.audio, d.format.audio));
                            download_format.subtitles.extend(d.subtitles)
                        } else {
                            download_formats.insert(
                                d.duration,
                                DownloadFormat {
                                    video: (d.video, d.format.audio.clone()),
                                    audios: vec![(d.audio, d.format.audio)],
                                    subtitles: d.subtitles,
                                },
                            );
                        }
                    }

                    for download_format in download_formats.into_values() {
                        downloader.add_format(download_format)
                    }
                }
            }

            result.push((downloader, format))
        }

        Ok(result)
    }
}

fn missing_locales<'a>(available: &Vec<Locale>, searched: &'a Vec<Locale>) -> Vec<&'a Locale> {
    searched.iter().filter(|p| !available.contains(p)).collect()
}
