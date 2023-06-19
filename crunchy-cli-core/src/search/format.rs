use crate::search::filter::FilterOptions;
use anyhow::{bail, Result};
use crunchyroll_rs::media::{Stream, Subtitle};
use crunchyroll_rs::{
    Concert, Episode, Locale, MediaCollection, Movie, MovieListing, MusicVideo, Season, Series,
};
use regex::Regex;
use serde::Serialize;
use serde_json::{Map, Value};
use std::collections::HashMap;
use std::ops::Range;

#[derive(Default, Serialize)]
struct FormatSeries {
    pub title: String,
    pub description: String,
}

impl From<&Series> for FormatSeries {
    fn from(value: &Series) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
        }
    }
}

#[derive(Default, Serialize)]
struct FormatSeason {
    pub title: String,
    pub description: String,
    pub number: u32,
}

impl From<&Season> for FormatSeason {
    fn from(value: &Season) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
            number: value.season_number,
        }
    }
}

#[derive(Default, Serialize)]
struct FormatEpisode {
    pub title: String,
    pub description: String,
    pub locale: Locale,
    pub number: u32,
    pub sequence_number: f32,
}

impl From<&Episode> for FormatEpisode {
    fn from(value: &Episode) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
            locale: value.audio_locale.clone(),
            number: value.episode_number,
            sequence_number: value.sequence_number,
        }
    }
}

#[derive(Default, Serialize)]
struct FormatMovieListing {
    pub title: String,
    pub description: String,
}

impl From<&MovieListing> for FormatMovieListing {
    fn from(value: &MovieListing) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
        }
    }
}

#[derive(Default, Serialize)]
struct FormatMovie {
    pub title: String,
    pub description: String,
}

impl From<&Movie> for FormatMovie {
    fn from(value: &Movie) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
        }
    }
}

#[derive(Default, Serialize)]
struct FormatMusicVideo {
    pub title: String,
    pub description: String,
}

impl From<&MusicVideo> for FormatMusicVideo {
    fn from(value: &MusicVideo) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
        }
    }
}

#[derive(Default, Serialize)]
struct FormatConcert {
    pub title: String,
    pub description: String,
}

impl From<&Concert> for FormatConcert {
    fn from(value: &Concert) -> Self {
        Self {
            title: value.title.clone(),
            description: value.description.clone(),
        }
    }
}

#[derive(Default, Serialize)]
struct FormatStream {
    pub locale: Locale,
    pub dash_url: String,
    pub hls_url: String,
}

impl From<&Stream> for FormatStream {
    fn from(value: &Stream) -> Self {
        let (dash_url, hls_url) = value.variants.get(&Locale::Custom("".to_string())).map_or(
            ("".to_string(), "".to_string()),
            |v| {
                (
                    v.adaptive_dash.clone().unwrap_or_default().url,
                    v.adaptive_hls.clone().unwrap_or_default().url,
                )
            },
        );

        Self {
            locale: value.audio_locale.clone(),
            dash_url,
            hls_url,
        }
    }
}

#[derive(Default, Serialize)]
struct FormatSubtitle {
    pub locale: Locale,
    pub url: String,
}

impl From<&Subtitle> for FormatSubtitle {
    fn from(value: &Subtitle) -> Self {
        Self {
            locale: value.locale.clone(),
            url: value.url.clone(),
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Hash)]
enum Scope {
    Series,
    Season,
    Episode,
    MovieListing,
    Movie,
    MusicVideo,
    Concert,
    Stream,
    Subtitle,
}

macro_rules! must_match_if_true {
    ($condition:expr => $media_collection:ident | $field:pat => $expr:expr) => {
        if $condition {
            match &$media_collection {
                $field => Some($expr),
                _ => panic!(),
            }
        } else {
            None
        }
    };
}

macro_rules! self_and_versions {
    ($var:expr => $audio:expr) => {{
        let mut items = vec![$var.clone()];
        items.extend($var.clone().version($audio).await?);
        items
    }};
}

pub struct Format {
    pattern: Vec<(Range<usize>, Scope, String)>,
    pattern_count: HashMap<Scope, u32>,
    input: String,
    filter_options: FilterOptions,
}

impl Format {
    pub fn new(input: String, filter_options: FilterOptions) -> Result<Self> {
        let scope_regex = Regex::new(r"(?m)\{\{\s*(?P<scope>\w+)\.(?P<field>\w+)\s*}}").unwrap();
        let mut pattern = vec![];
        let mut pattern_count = HashMap::new();

        macro_rules! generate_field_check {
            ($($scope:expr => $struct_:ident)+) => {
                HashMap::from([
                    $(
                        (
                            $scope,
                            serde_json::from_value::<Map<String, Value>>(serde_json::to_value($struct_::default()).unwrap()).unwrap()
                        )
                    ),+
                ])
            };
        }
        let field_check = generate_field_check!(
            Scope::Series => FormatSeries
            Scope::Season => FormatSeason
            Scope::Episode => FormatEpisode
            Scope::MovieListing => FormatMovieListing
            Scope::Movie => FormatMovie
            Scope::MusicVideo => FormatMusicVideo
            Scope::Concert => FormatConcert
            Scope::Stream => FormatStream
            Scope::Subtitle => FormatSubtitle
        );

        for capture in scope_regex.captures_iter(&input) {
            let full = capture.get(0).unwrap();
            let scope = capture.name("scope").unwrap().as_str();
            let field = capture.name("field").unwrap().as_str();

            let format_pattern_scope = match scope {
                "series" => Scope::Series,
                "season" => Scope::Season,
                "episode" => Scope::Episode,
                "movie_listing" => Scope::MovieListing,
                "movie" => Scope::Movie,
                "music_video" => Scope::MusicVideo,
                "concert" => Scope::Concert,
                "stream" => Scope::Stream,
                "subtitle" => Scope::Subtitle,
                _ => bail!("'{}.{}' is not a valid keyword", scope, field),
            };

            if field_check
                .get(&format_pattern_scope)
                .unwrap()
                .get(field)
                .is_none()
            {
                bail!("'{}.{}' is not a valid keyword", scope, field)
            }

            pattern.push((
                full.start()..full.end(),
                format_pattern_scope.clone(),
                field.to_string(),
            ));
            *pattern_count.entry(format_pattern_scope).or_default() += 1
        }

        Ok(Self {
            pattern,
            pattern_count,
            input,
            filter_options,
        })
    }

    pub async fn parse(&self, media_collection: MediaCollection) -> Result<String> {
        match &media_collection {
            MediaCollection::Series(_)
            | MediaCollection::Season(_)
            | MediaCollection::Episode(_) => {
                self.check_scopes(vec![
                    Scope::Series,
                    Scope::Season,
                    Scope::Episode,
                    Scope::Stream,
                    Scope::Subtitle,
                ])?;

                self.parse_series(media_collection).await
            }
            MediaCollection::MovieListing(_) | MediaCollection::Movie(_) => {
                self.check_scopes(vec![
                    Scope::MovieListing,
                    Scope::Movie,
                    Scope::Stream,
                    Scope::Subtitle,
                ])?;

                self.parse_movie_listing(media_collection).await
            }
            MediaCollection::MusicVideo(_) => {
                self.check_scopes(vec![Scope::MusicVideo, Scope::Stream, Scope::Subtitle])?;

                self.parse_music_video(media_collection).await
            }
            MediaCollection::Concert(_) => {
                self.check_scopes(vec![Scope::Concert, Scope::Stream, Scope::Subtitle])?;

                self.parse_concert(media_collection).await
            }
        }
    }

    async fn parse_series(&self, media_collection: MediaCollection) -> Result<String> {
        let series_empty = self.check_pattern_count_empty(Scope::Series);
        let season_empty = self.check_pattern_count_empty(Scope::Season);
        let episode_empty = self.check_pattern_count_empty(Scope::Episode);
        let stream_empty = self.check_pattern_count_empty(Scope::Stream)
            && self.check_pattern_count_empty(Scope::Subtitle);

        let mut tree: Vec<(Season, Vec<(Episode, Vec<Stream>)>)> = vec![];

        let series = if !series_empty {
            let series = match &media_collection {
                MediaCollection::Series(series) => series.clone(),
                MediaCollection::Season(season) => season.series().await?,
                MediaCollection::Episode(episode) => episode.series().await?,
                _ => panic!(),
            };
            if !self.filter_options.check_series(&series) {
                return Ok("".to_string());
            }
            series
        } else {
            Series::default()
        };
        if !season_empty || !episode_empty || !stream_empty {
            let tmp_seasons = match &media_collection {
                MediaCollection::Series(series) => series.seasons().await?,
                MediaCollection::Season(season) => vec![season.clone()],
                MediaCollection::Episode(_) => vec![],
                _ => panic!(),
            };
            let mut seasons = vec![];
            for season in tmp_seasons {
                seasons.extend(self_and_versions!(season => self.filter_options.audio.clone()))
            }
            tree.extend(
                self.filter_options
                    .filter_seasons(seasons)
                    .into_iter()
                    .map(|s| (s, vec![])),
            )
        } else {
            tree.push((Season::default(), vec![]))
        }
        if !episode_empty || !stream_empty {
            match &media_collection {
                MediaCollection::Episode(episode) => {
                    let episodes = self_and_versions!(episode => self.filter_options.audio.clone());
                    tree.push((
                        Season::default(),
                        episodes
                            .into_iter()
                            .filter(|e| self.filter_options.audio.contains(&e.audio_locale))
                            .map(|e| (e, vec![]))
                            .collect(),
                    ))
                }
                _ => {
                    for (season, episodes) in tree.iter_mut() {
                        episodes.extend(
                            self.filter_options
                                .filter_episodes(season.episodes().await?)
                                .into_iter()
                                .map(|e| (e, vec![])),
                        )
                    }
                }
            };
        } else {
            for (_, episodes) in tree.iter_mut() {
                episodes.push((Episode::default(), vec![]))
            }
        }
        if !stream_empty {
            for (_, episodes) in tree.iter_mut() {
                for (episode, streams) in episodes {
                    streams.push(episode.streams().await?)
                }
            }
        } else {
            for (_, episodes) in tree.iter_mut() {
                for (_, streams) in episodes {
                    streams.push(Stream::default())
                }
            }
        }

        let mut output = vec![];
        let series_map = self.serializable_to_json_map(FormatSeries::from(&series));
        for (season, episodes) in tree {
            let season_map = self.serializable_to_json_map(FormatSeason::from(&season));
            for (episode, streams) in episodes {
                let episode_map = self.serializable_to_json_map(FormatEpisode::from(&episode));
                for stream in streams {
                    let stream_map = self.serializable_to_json_map(FormatStream::from(&stream));

                    output.push(
                        self.replace_all(
                            HashMap::from([
                                (Scope::Series, &series_map),
                                (Scope::Season, &season_map),
                                (Scope::Episode, &episode_map),
                                (Scope::Stream, &stream_map),
                            ]),
                            stream,
                        )
                        .unwrap_or_default(),
                    )
                }
            }
        }

        Ok(output.join("\n"))
    }

    async fn parse_movie_listing(&self, media_collection: MediaCollection) -> Result<String> {
        let movie_listing_empty = self.check_pattern_count_empty(Scope::MovieListing);
        let movie_empty = self.check_pattern_count_empty(Scope::Movie);
        let stream_empty = self.check_pattern_count_empty(Scope::Stream);

        let mut tree: Vec<(Movie, Vec<Stream>)> = vec![];

        let movie_listing = if !movie_listing_empty {
            let movie_listing = match &media_collection {
                MediaCollection::MovieListing(movie_listing) => movie_listing.clone(),
                MediaCollection::Movie(movie) => movie.movie_listing().await?,
                _ => panic!(),
            };
            if !self.filter_options.check_movie_listing(&movie_listing) {
                return Ok("".to_string());
            }
            movie_listing
        } else {
            MovieListing::default()
        };
        if !movie_empty || !stream_empty {
            let movies = match &media_collection {
                MediaCollection::MovieListing(movie_listing) => movie_listing.movies().await?,
                MediaCollection::Movie(movie) => vec![movie.clone()],
                _ => panic!(),
            };
            tree.extend(movies.into_iter().map(|m| (m, vec![])))
        }
        if !stream_empty {
            for (movie, streams) in tree.iter_mut() {
                streams.push(movie.streams().await?)
            }
        } else {
            for (_, streams) in tree.iter_mut() {
                streams.push(Stream::default())
            }
        }

        let mut output = vec![];
        let movie_listing_map =
            self.serializable_to_json_map(FormatMovieListing::from(&movie_listing));
        for (movie, streams) in tree {
            let movie_map = self.serializable_to_json_map(FormatMovie::from(&movie));
            for stream in streams {
                let stream_map = self.serializable_to_json_map(FormatStream::from(&stream));

                output.push(
                    self.replace_all(
                        HashMap::from([
                            (Scope::MovieListing, &movie_listing_map),
                            (Scope::Movie, &movie_map),
                            (Scope::Stream, &stream_map),
                        ]),
                        stream,
                    )
                    .unwrap_or_default(),
                )
            }
        }

        Ok(output.join("\n"))
    }

    async fn parse_music_video(&self, media_collection: MediaCollection) -> Result<String> {
        let music_video_empty = self.check_pattern_count_empty(Scope::MusicVideo);
        let stream_empty = self.check_pattern_count_empty(Scope::Stream);

        let music_video = must_match_if_true!(!music_video_empty => media_collection|MediaCollection::MusicVideo(music_video) => music_video.clone()).unwrap_or_default();
        let stream = must_match_if_true!(!stream_empty => media_collection|MediaCollection::MusicVideo(music_video) => music_video.streams().await?).unwrap_or_default();

        let music_video_map = self.serializable_to_json_map(FormatMusicVideo::from(&music_video));
        let stream_map = self.serializable_to_json_map(FormatStream::from(&stream));

        let output = self
            .replace_all(
                HashMap::from([
                    (Scope::MusicVideo, &music_video_map),
                    (Scope::Stream, &stream_map),
                ]),
                stream,
            )
            .unwrap_or_default();
        Ok(output)
    }

    async fn parse_concert(&self, media_collection: MediaCollection) -> Result<String> {
        let concert_empty = self.check_pattern_count_empty(Scope::Concert);
        let stream_empty = self.check_pattern_count_empty(Scope::Stream);

        let concert = must_match_if_true!(!concert_empty => media_collection|MediaCollection::Concert(concert) => concert.clone()).unwrap_or_default();
        let stream = must_match_if_true!(!stream_empty => media_collection|MediaCollection::Concert(concert) => concert.streams().await?).unwrap_or_default();

        let concert_map = self.serializable_to_json_map(FormatConcert::from(&concert));
        let stream_map = self.serializable_to_json_map(FormatStream::from(&stream));

        let output = self
            .replace_all(
                HashMap::from([(Scope::Concert, &concert_map), (Scope::Stream, &stream_map)]),
                stream,
            )
            .unwrap_or_default();
        Ok(output)
    }

    fn serializable_to_json_map<S: Serialize>(&self, s: S) -> Map<String, Value> {
        serde_json::from_value(serde_json::to_value(s).unwrap()).unwrap()
    }

    fn check_pattern_count_empty(&self, scope: Scope) -> bool {
        self.pattern_count.get(&scope).cloned().unwrap_or_default() == 0
    }

    fn check_scopes(&self, available_scopes: Vec<Scope>) -> Result<()> {
        for (_, scope, field) in self.pattern.iter() {
            if !available_scopes.contains(scope) {
                bail!(
                    "'{}.{}' is not a valid keyword",
                    format!("{:?}", scope).to_lowercase(),
                    field
                )
            }
        }
        Ok(())
    }

    fn replace_all(
        &self,
        values: HashMap<Scope, &Map<String, Value>>,
        mut stream: Stream,
    ) -> Option<String> {
        if stream.subtitles.is_empty() {
            if !self.check_pattern_count_empty(Scope::Subtitle) {
                return None;
            }
            stream
                .subtitles
                .insert(Locale::Custom("".to_string()), Subtitle::default());
        }
        let subtitles = self
            .filter_options
            .filter_subtitles(stream.subtitles.into_values().collect());

        let mut output = vec![];
        for subtitle in subtitles {
            let subtitle_map = self.serializable_to_json_map(FormatSubtitle::from(&subtitle));
            let mut tmp_values = values.clone();
            tmp_values.insert(Scope::Subtitle, &subtitle_map);
            output.push(self.replace(tmp_values))
        }

        Some(output.join("\n"))
    }

    fn replace(&self, values: HashMap<Scope, &Map<String, Value>>) -> String {
        let mut output = self.input.clone();
        let mut offset = 0;
        for (range, scope, field) in &self.pattern {
            let item =
                serde_plain::to_string(values.get(scope).unwrap().get(field.as_str()).unwrap())
                    .unwrap();
            let start = (range.start as i32 + offset) as usize;
            let end = (range.end as i32 + offset) as usize;
            output.replace_range(start..end, &item);
            offset += item.len() as i32 - range.len() as i32;
        }

        output
    }
}
