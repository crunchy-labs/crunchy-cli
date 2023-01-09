use crunchyroll_rs::media::VariantData;
use crunchyroll_rs::{Episode, Locale, Media, Movie};
use log::warn;
use std::path::PathBuf;
use std::time::Duration;

#[derive(Clone)]
pub struct Format {
    pub title: String,
    pub description: String,

    pub audio: Locale,

    pub duration: Duration,
    pub stream: VariantData,

    pub series_id: String,
    pub series_name: String,

    pub season_id: String,
    pub season_title: String,
    pub season_number: u32,

    pub episode_id: String,
    pub episode_number: f32,
    pub relative_episode_number: f32,
}

impl Format {
    pub fn new_from_episode(
        episode: &Media<Episode>,
        season_episodes: &Vec<Media<Episode>>,
        stream: VariantData,
    ) -> Self {
        Self {
            title: episode.title.clone(),
            description: episode.description.clone(),

            audio: episode.metadata.audio_locale.clone(),

            duration: episode.metadata.duration.to_std().unwrap(),
            stream,

            series_id: episode.metadata.series_id.clone(),
            series_name: episode.metadata.series_title.clone(),

            season_id: episode.metadata.season_id.clone(),
            season_title: episode.metadata.season_title.clone(),
            season_number: episode.metadata.season_number.clone(),

            episode_id: episode.id.clone(),
            episode_number: episode
                .metadata
                .episode
                .parse()
                .unwrap_or(episode.metadata.sequence_number),
            relative_episode_number: season_episodes
                .iter()
                .enumerate()
                .find_map(|(i, e)| if e == episode { Some((i + 1) as f32) } else { None })
                .unwrap_or_else(|| {
                    warn!("Cannot find relative episode number for episode {} ({}) of season {} ({}) of {}, using normal episode number", episode.metadata.episode_number, episode.title, episode.metadata.season_number, episode.metadata.season_title, episode.metadata.series_title);
                    episode
                        .metadata
                        .episode
                        .parse()
                        .unwrap_or(episode.metadata.sequence_number)
                }),
        }
    }

    pub fn new_from_movie(movie: &Media<Movie>, stream: VariantData) -> Self {
        Self {
            title: movie.title.clone(),
            description: movie.description.clone(),

            audio: Locale::ja_JP,

            duration: movie.metadata.duration.to_std().unwrap(),
            stream,

            series_id: movie.metadata.movie_listing_id.clone(),
            series_name: movie.metadata.movie_listing_title.clone(),

            season_id: movie.metadata.movie_listing_id.clone(),
            season_title: movie.metadata.movie_listing_title.clone(),
            season_number: 1,

            episode_id: movie.id.clone(),
            episode_number: 1.0,
            relative_episode_number: 1.0,
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
                .replace("{audio}", &sanitize_func(&self.audio.to_string()))
                .replace(
                    "{resolution}",
                    &sanitize_func(&self.stream.resolution.to_string()),
                )
                .replace("{series_id}", &sanitize_func(&self.series_id))
                .replace("{series_name}", &sanitize_func(&self.series_name))
                .replace("{season_id}", &sanitize_func(&self.season_id))
                .replace("{season_name}", &sanitize_func(&self.season_title))
                .replace(
                    "{season_number}",
                    &sanitize_func(&self.season_number.to_string()),
                )
                .replace(
                    "{padded_season_number}",
                    &sanitize_func(&format!("{:0>2}", self.season_number.to_string())),
                )
                .replace("{episode_id}", &sanitize_func(&self.episode_id))
                .replace(
                    "{episode_number}",
                    &sanitize_func(&self.episode_number.to_string()),
                )
                .replace(
                    "{padded_episode_number}",
                    &sanitize_func(&format!("{:0>2}", self.episode_number.to_string())),
                )
                .replace(
                    "{relative_episode_number}",
                    &sanitize_func(&format!("{:0>2}", self.relative_episode_number.to_string())),
                ),
        )
    }

    pub fn has_relative_episodes_fmt<S: AsRef<str>>(s: S) -> bool {
        return s.as_ref().contains("{relative_episode_number}");
    }
}
