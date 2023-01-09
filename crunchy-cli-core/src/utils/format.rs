use crunchyroll_rs::media::VariantData;
use crunchyroll_rs::{Episode, Locale, Media, Movie};
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
}

impl Format {
    pub fn new_from_episode(episode: Media<Episode>, stream: VariantData) -> Self {
        Self {
            title: episode.title,
            description: episode.description,

            audio: episode.metadata.audio_locale,

            duration: episode.metadata.duration.to_std().unwrap(),
            stream,

            series_id: episode.metadata.series_id,
            series_name: episode.metadata.series_title,

            season_id: episode.metadata.season_id,
            season_title: episode.metadata.season_title,
            season_number: episode.metadata.season_number,

            episode_id: episode.id,
            episode_number: episode.metadata.episode.parse().unwrap_or(episode.metadata.sequence_number),
        }
    }

    pub fn new_from_movie(movie: Media<Movie>, stream: VariantData) -> Self {
        Self {
            title: movie.title,
            description: movie.description,

            audio: Locale::ja_JP,

            duration: movie.metadata.duration.to_std().unwrap(),
            stream,

            series_id: movie.metadata.movie_listing_id.clone(),
            series_name: movie.metadata.movie_listing_title.clone(),

            season_id: movie.metadata.movie_listing_id,
            season_title: movie.metadata.movie_listing_title,
            season_number: 1,

            episode_id: movie.id,
            episode_number: 1.0,
        }
    }
}

/// Formats the given string if it has specific pattern in it. It's possible to sanitize it which
/// removes characters which can cause failures if the output string is used as a file name.
pub fn format_path(path: PathBuf, format: &Format, sanitize: bool) -> PathBuf {
    let sanitize_func = if sanitize {
        |s: &str| sanitize_filename::sanitize(s)
    } else {
        // converting this to a string is actually unnecessary
        |s: &str| s.to_string()
    };

    let as_string = path.to_string_lossy().to_string();

    PathBuf::from(
        as_string
            .replace("{title}", &sanitize_func(&format.title))
            .replace("{audio}", &sanitize_func(&format.audio.to_string()))
            .replace(
                "{resolution}",
                &sanitize_func(&format.stream.resolution.to_string()),
            )
            .replace("{series_id}", &sanitize_func(&format.series_id))
            .replace("{series_name}", &sanitize_func(&format.series_name))
            .replace("{season_id}", &sanitize_func(&format.season_id))
            .replace("{season_name}", &sanitize_func(&format.season_title))
            .replace(
                "{season_number}",
                &sanitize_func(&format.season_number.to_string()),
            )
            .replace(
                "{padded_season_number}",
                &sanitize_func(&format!("{:0>2}", format.season_number.to_string())),
            )
            .replace("{episode_id}", &sanitize_func(&format.episode_id))
            .replace("{episode_number}", &sanitize_func(&format.episode_number.to_string()))
            .replace(
                "{padded_episode_number}",
                &sanitize_func(&format!("{:0>2}", format.episode_number.to_string())),
            ),
    )
}
