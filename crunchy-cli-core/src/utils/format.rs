use crate::utils::filter::real_dedup_vec;
use crate::utils::log::tab_info;
use crate::utils::os::is_special_file;
use crunchyroll_rs::media::{Resolution, VariantData};
use crunchyroll_rs::{Concert, Episode, Locale, Movie, MusicVideo};
use log::{debug, info};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

#[derive(Clone)]
pub struct SingleFormat {
    pub title: String,
    pub description: String,

    pub audio: Locale,
    pub subtitles: Vec<Locale>,

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

impl SingleFormat {
    pub fn new_from_episode(
        episode: &Episode,
        video: &VariantData,
        subtitles: Vec<Locale>,
        relative_episode_number: Option<u32>,
    ) -> Self {
        Self {
            title: episode.title.clone(),
            description: episode.description.clone(),
            audio: episode.audio_locale.clone(),
            subtitles,
            resolution: video.resolution.clone(),
            fps: video.fps,
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
        }
    }

    pub fn new_from_movie(movie: &Movie, video: &VariantData, subtitles: Vec<Locale>) -> Self {
        Self {
            title: movie.title.clone(),
            description: movie.description.clone(),
            audio: Locale::ja_JP,
            subtitles,
            resolution: video.resolution.clone(),
            fps: video.fps,
            series_id: movie.movie_listing_id.clone(),
            series_name: movie.movie_listing_title.clone(),
            season_id: movie.movie_listing_id.clone(),
            season_title: movie.movie_listing_title.to_string(),
            season_number: 1,
            episode_id: movie.id.clone(),
            episode_number: "1".to_string(),
            sequence_number: 1.0,
            relative_episode_number: Some(1),
        }
    }

    pub fn new_from_music_video(music_video: &MusicVideo, video: &VariantData) -> Self {
        Self {
            title: music_video.title.clone(),
            description: music_video.description.clone(),
            audio: Locale::ja_JP,
            subtitles: vec![],
            resolution: video.resolution.clone(),
            fps: video.fps,
            series_id: music_video.id.clone(),
            series_name: music_video.title.clone(),
            season_id: music_video.id.clone(),
            season_title: music_video.title.clone(),
            season_number: 1,
            episode_id: music_video.id.clone(),
            episode_number: "1".to_string(),
            sequence_number: 1.0,
            relative_episode_number: Some(1),
        }
    }

    pub fn new_from_concert(concert: &Concert, video: &VariantData) -> Self {
        Self {
            title: concert.title.clone(),
            description: concert.description.clone(),
            audio: Locale::ja_JP,
            subtitles: vec![],
            resolution: video.resolution.clone(),
            fps: video.fps,
            series_id: concert.id.clone(),
            series_name: concert.title.clone(),
            season_id: concert.id.clone(),
            season_title: concert.title.clone(),
            season_number: 1,
            episode_id: concert.id.clone(),
            episode_number: "1".to_string(),
            sequence_number: 1.0,
            relative_episode_number: Some(1),
        }
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
    pub fn from_single_formats(mut single_formats: Vec<SingleFormat>) -> Self {
        let locales: Vec<(Locale, Vec<Locale>)> = single_formats
            .iter()
            .map(|sf| (sf.audio.clone(), sf.subtitles.clone()))
            .collect();
        let first = single_formats.remove(0);

        Self {
            title: first.title,
            description: first.description,
            locales,
            resolution: first.resolution,
            fps: first.fps,
            series_id: first.series_id,
            series_name: first.series_name,
            season_id: first.season_id,
            season_title: first.season_title,
            season_number: first.season_number,
            episode_id: first.episode_id,
            episode_number: first.episode_number,
            sequence_number: first.sequence_number,
            relative_episode_number: first.relative_episode_number,
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

pub fn formats_visual_output(formats: Vec<&Format>) {
    if log::max_level() == log::Level::Debug {
        let seasons = sort_formats_after_seasons(formats);
        debug!("Series has {} seasons", seasons.len());
        for (i, season) in seasons.into_iter().enumerate() {
            info!("Season {}", i + 1);
            for format in season {
                info!(
                    "{}: {}px, {:.02} FPS (S{:02}E{:0>2})",
                    format.title,
                    format.resolution,
                    format.fps,
                    format.season_number,
                    format.episode_number,
                )
            }
        }
    } else {
        for season in sort_formats_after_seasons(formats) {
            let first = season.get(0).unwrap();
            info!("{} Season {}", first.series_name, first.season_number);

            for (i, format) in season.into_iter().enumerate() {
                tab_info!(
                    "{}. {} » {}px, {:.2} FPS (S{:02}E{:0>2})",
                    i + 1,
                    format.title,
                    format.resolution,
                    format.fps,
                    format.season_number,
                    format.episode_number
                )
            }
        }
    }
}

fn sort_formats_after_seasons(formats: Vec<&Format>) -> Vec<Vec<&Format>> {
    let mut season_map = BTreeMap::new();

    for format in formats {
        season_map
            .entry(format.season_number)
            .or_insert(vec![])
            .push(format)
    }

    season_map
        .into_values()
        .into_iter()
        .map(|mut fmts| {
            fmts.sort_by(|a, b| a.sequence_number.total_cmp(&b.sequence_number));
            fmts
        })
        .collect()
}
