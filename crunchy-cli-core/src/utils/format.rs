use crate::utils::filter::real_dedup_vec;
use crate::utils::log::tab_info;
use crate::utils::os::is_special_file;
use anyhow::Result;
use chrono::Duration;
use crunchyroll_rs::media::{Resolution, Stream, Subtitle, VariantData};
use crunchyroll_rs::{Concert, Episode, Locale, MediaCollection, Movie, MusicVideo};
use log::{debug, info};
use std::cmp::Ordering;
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

#[derive(Clone)]
pub struct SingleFormat {
    pub title: String,
    pub description: String,

    pub audio: Locale,
    pub subtitles: Vec<Locale>,

    pub series_id: String,
    pub series_name: String,

    pub season_id: String,
    pub season_title: String,
    pub season_number: u32,

    pub episode_id: String,
    pub episode_number: String,
    pub sequence_number: f32,
    pub relative_episode_number: Option<u32>,

    pub duration: Duration,

    source: MediaCollection,
}

impl SingleFormat {
    pub fn new_from_episode(
        episode: Episode,
        subtitles: Vec<Locale>,
        relative_episode_number: Option<u32>,
    ) -> Self {
        Self {
            title: episode.title.clone(),
            description: episode.description.clone(),
            audio: episode.audio_locale.clone(),
            subtitles,
            series_id: episode.series_id.clone(),
            series_name: episode.series_title.clone(),
            season_id: episode.season_id.clone(),
            season_title: episode.season_title.to_string(),
            season_number: episode.season_number,
            episode_id: episode.id.clone(),
            episode_number: if episode.episode.is_empty() {
                episode.sequence_number.to_string()
            } else {
                episode.episode.clone()
            },
            sequence_number: episode.sequence_number,
            relative_episode_number,
            duration: episode.duration,
            source: episode.into(),
        }
    }

    pub fn new_from_movie(movie: Movie, subtitles: Vec<Locale>) -> Self {
        Self {
            title: movie.title.clone(),
            description: movie.description.clone(),
            audio: Locale::ja_JP,
            subtitles,
            series_id: movie.movie_listing_id.clone(),
            series_name: movie.movie_listing_title.clone(),
            season_id: movie.movie_listing_id.clone(),
            season_title: movie.movie_listing_title.to_string(),
            season_number: 1,
            episode_id: movie.id.clone(),
            episode_number: "1".to_string(),
            sequence_number: 1.0,
            relative_episode_number: Some(1),
            duration: movie.duration,
            source: movie.into(),
        }
    }

    pub fn new_from_music_video(music_video: MusicVideo) -> Self {
        Self {
            title: music_video.title.clone(),
            description: music_video.description.clone(),
            audio: Locale::ja_JP,
            subtitles: vec![],
            series_id: music_video.id.clone(),
            series_name: music_video.title.clone(),
            season_id: music_video.id.clone(),
            season_title: music_video.title.clone(),
            season_number: 1,
            episode_id: music_video.id.clone(),
            episode_number: "1".to_string(),
            sequence_number: 1.0,
            relative_episode_number: Some(1),
            duration: music_video.duration,
            source: music_video.into(),
        }
    }

    pub fn new_from_concert(concert: Concert) -> Self {
        Self {
            title: concert.title.clone(),
            description: concert.description.clone(),
            audio: Locale::ja_JP,
            subtitles: vec![],
            series_id: concert.id.clone(),
            series_name: concert.title.clone(),
            season_id: concert.id.clone(),
            season_title: concert.title.clone(),
            season_number: 1,
            episode_id: concert.id.clone(),
            episode_number: "1".to_string(),
            sequence_number: 1.0,
            relative_episode_number: Some(1),
            duration: concert.duration,
            source: concert.into(),
        }
    }

    pub async fn stream(&self) -> Result<Stream> {
        let stream = match &self.source {
            MediaCollection::Episode(e) => e.streams().await?,
            MediaCollection::Movie(m) => m.streams().await?,
            MediaCollection::MusicVideo(mv) => mv.streams().await?,
            MediaCollection::Concert(c) => c.streams().await?,
            _ => unreachable!(),
        };
        Ok(stream)
    }

    pub fn source_type(&self) -> String {
        match &self.source {
            MediaCollection::Episode(_) => "episode",
            MediaCollection::Movie(_) => "movie",
            MediaCollection::MusicVideo(_) => "music video",
            MediaCollection::Concert(_) => "concert",
            _ => unreachable!(),
        }
        .to_string()
    }

    pub fn is_episode(&self) -> bool {
        match self.source {
            MediaCollection::Episode(_) => true,
            _ => false,
        }
    }
}

struct SingleFormatCollectionEpisodeKey(f32);

impl PartialOrd for SingleFormatCollectionEpisodeKey {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        self.0.partial_cmp(&other.0)
    }
}
impl Ord for SingleFormatCollectionEpisodeKey {
    fn cmp(&self, other: &Self) -> Ordering {
        self.0.total_cmp(&other.0)
    }
}
impl PartialEq for SingleFormatCollectionEpisodeKey {
    fn eq(&self, other: &Self) -> bool {
        self.0.eq(&other.0)
    }
}
impl Eq for SingleFormatCollectionEpisodeKey {}

pub struct SingleFormatCollection(
    BTreeMap<u32, BTreeMap<SingleFormatCollectionEpisodeKey, Vec<SingleFormat>>>,
);

impl SingleFormatCollection {
    pub fn new() -> Self {
        Self(BTreeMap::new())
    }

    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    pub fn add_single_formats(&mut self, single_formats: Vec<SingleFormat>) {
        let format = single_formats.first().unwrap();
        self.0
            .entry(format.season_number)
            .or_insert(BTreeMap::new())
            .insert(
                SingleFormatCollectionEpisodeKey(format.sequence_number),
                single_formats,
            );
    }

    pub fn full_visual_output(&self) {
        debug!("Series has {} seasons", self.0.len());
        for (season_number, episodes) in &self.0 {
            info!(
                "{} Season {}",
                episodes
                    .first_key_value()
                    .unwrap()
                    .1
                    .first()
                    .unwrap()
                    .series_name
                    .clone(),
                season_number
            );
            for (i, (_, formats)) in episodes.iter().enumerate() {
                let format = formats.first().unwrap();
                if log::max_level() == log::Level::Debug {
                    info!(
                        "{} S{:02}E{:0>2}",
                        format.title, format.season_number, format.episode_number
                    )
                } else {
                    tab_info!(
                        "{}. {} » S{:02}E{:0>2}",
                        i + 1,
                        format.title,
                        format.season_number,
                        format.episode_number
                    )
                }
            }
        }
    }
}

impl IntoIterator for SingleFormatCollection {
    type Item = Vec<SingleFormat>;
    type IntoIter = SingleFormatCollectionIterator;

    fn into_iter(self) -> Self::IntoIter {
        SingleFormatCollectionIterator(self)
    }
}

pub struct SingleFormatCollectionIterator(SingleFormatCollection);

impl Iterator for SingleFormatCollectionIterator {
    type Item = Vec<SingleFormat>;

    fn next(&mut self) -> Option<Self::Item> {
        let Some((_, episodes)) = self.0.0.iter_mut().next() else {
            return None
        };

        let value = episodes.pop_first().unwrap().1;
        if episodes.is_empty() {
            self.0 .0.pop_first();
        }
        Some(value)
    }
}

#[derive(Clone)]
pub struct Format {
    pub title: String,
    pub description: String,

    pub locales: Vec<(Locale, Vec<Locale>)>,

    pub resolution: Resolution,
    pub fps: f64,

    pub series_id: String,
    pub series_name: String,

    pub season_id: String,
    pub season_title: String,
    pub season_number: u32,

    pub episode_id: String,
    pub episode_number: String,
    pub sequence_number: f32,
    pub relative_episode_number: Option<u32>,
}

impl Format {
    pub fn from_single_formats(
        mut single_formats: Vec<(SingleFormat, VariantData, Vec<Subtitle>)>,
    ) -> Self {
        let locales: Vec<(Locale, Vec<Locale>)> = single_formats
            .iter()
            .map(|(single_format, _, subtitles)| {
                (
                    single_format.audio.clone(),
                    subtitles
                        .into_iter()
                        .map(|s| s.locale.clone())
                        .collect::<Vec<Locale>>(),
                )
            })
            .collect();
        let (first_format, first_stream, _) = single_formats.remove(0);

        Self {
            title: first_format.title,
            description: first_format.description,
            locales,
            resolution: first_stream.resolution,
            fps: first_stream.fps,
            series_id: first_format.series_id,
            series_name: first_format.series_name,
            season_id: first_format.season_id,
            season_title: first_format.season_title,
            season_number: first_format.season_number,
            episode_id: first_format.episode_id,
            episode_number: first_format.episode_number,
            sequence_number: first_format.sequence_number,
            relative_episode_number: first_format.relative_episode_number,
        }
    }

    /// Formats the given string if it has specific pattern in it. It's possible to sanitize it which
    /// removes characters which can cause failures if the output string is used as a file name.
    pub fn format_path(&self, path: PathBuf, sanitize: bool) -> PathBuf {
        let sanitize_func = if sanitize {
            |s: &str| sanitize_filename::sanitize(s)
        } else {
            // converting this to a string is actually unnecessary
            |s: &str| s.to_string()
        };

        let as_string = path.to_string_lossy().to_string();

        PathBuf::from(
            as_string
                .replace("{title}", &sanitize_func(&self.title))
                .replace(
                    "{audio}",
                    &sanitize_func(
                        &self
                            .locales
                            .iter()
                            .map(|(a, _)| a.to_string())
                            .collect::<Vec<String>>()
                            .join("|"),
                    ),
                )
                .replace("{resolution}", &sanitize_func(&self.resolution.to_string()))
                .replace("{series_id}", &sanitize_func(&self.series_id))
                .replace("{series_name}", &sanitize_func(&self.series_name))
                .replace("{season_id}", &sanitize_func(&self.season_id))
                .replace("{season_name}", &sanitize_func(&self.season_title))
                .replace(
                    "{season_number}",
                    &sanitize_func(&format!("{:0>2}", self.season_number.to_string())),
                )
                .replace("{episode_id}", &sanitize_func(&self.episode_id))
                .replace(
                    "{episode_number}",
                    &sanitize_func(&format!("{:0>2}", self.episode_number.to_string())),
                )
                .replace(
                    "{relative_episode_number}",
                    &sanitize_func(&format!(
                        "{:0>2}",
                        self.relative_episode_number.unwrap_or_default().to_string()
                    )),
                ),
        )
    }

    pub fn visual_output(&self, dst: &Path) {
        info!(
            "Downloading {} to {}",
            self.title,
            if is_special_file(&dst) || dst.to_str().unwrap() == "-" {
                dst.to_string_lossy().to_string()
            } else {
                format!("'{}'", dst.to_str().unwrap())
            }
        );
        tab_info!(
            "Episode: S{:02}E{:0>2}",
            self.season_number,
            self.episode_number
        );
        tab_info!(
            "Audio: {}",
            self.locales
                .iter()
                .map(|(a, _)| a.to_string())
                .collect::<Vec<String>>()
                .join(", ")
        );
        let mut subtitles: Vec<Locale> = self.locales.iter().flat_map(|(_, s)| s.clone()).collect();
        real_dedup_vec(&mut subtitles);
        tab_info!(
            "Subtitles: {}",
            subtitles
                .into_iter()
                .map(|l| l.to_string())
                .collect::<Vec<String>>()
                .join(", ")
        );
        tab_info!("Resolution: {}", self.resolution);
        tab_info!("FPS: {:.2}", self.fps)
    }

    pub fn has_relative_episodes_fmt<S: AsRef<str>>(s: S) -> bool {
        return s.as_ref().contains("{relative_episode_number}");
    }
}
