use crate::utils::format::{SingleFormat, SingleFormatCollection};
use crate::utils::interactive_select::{check_for_duplicated_seasons, get_duplicated_seasons};
use crate::utils::parse::{fract, UrlFilter};
use anyhow::Result;
use crunchyroll_rs::{
    Concert, Episode, Locale, MediaCollection, Movie, MovieListing, MusicVideo, Season, Series,
};
use log::{info, warn};
use std::collections::{BTreeMap, HashMap};
use std::ops::Not;

pub(crate) enum FilterMediaScope<'a> {
    Series(&'a Series),
    Season(&'a Season),
    /// Always contains 1 or 2 episodes.
    /// - 1: The episode's audio is completely missing
    /// - 2: The requested audio is only available from first entry to last entry
    Episode(Vec<&'a Episode>),
}

pub(crate) struct Filter {
    url_filter: UrlFilter,

    skip_specials: bool,
    interactive_input: bool,

    relative_episode_number: bool,

    audio_locales: Vec<Locale>,
    subtitle_locales: Vec<Locale>,

    audios_missing: fn(FilterMediaScope, Vec<&Locale>) -> Result<bool>,
    subtitles_missing: fn(FilterMediaScope, Vec<&Locale>) -> Result<bool>,
    no_premium: fn(u32) -> Result<()>,

    is_premium: bool,

    series_visited: bool,
    season_episodes: HashMap<String, Vec<Episode>>,
    season_with_premium: Option<Vec<u32>>,
    season_sorting: Vec<String>,
}

impl Filter {
    #[allow(clippy::too_many_arguments)]
    pub(crate) fn new(
        url_filter: UrlFilter,
        audio_locales: Vec<Locale>,
        subtitle_locales: Vec<Locale>,
        audios_missing: fn(FilterMediaScope, Vec<&Locale>) -> Result<bool>,
        subtitles_missing: fn(FilterMediaScope, Vec<&Locale>) -> Result<bool>,
        no_premium: fn(u32) -> Result<()>,
        relative_episode_number: bool,
        interactive_input: bool,
        skip_specials: bool,
        is_premium: bool,
    ) -> Self {
        Self {
            url_filter,
            audio_locales,
            subtitle_locales,
            relative_episode_number,
            interactive_input,
            audios_missing,
            subtitles_missing,
            no_premium,
            is_premium,
            series_visited: false,
            season_episodes: HashMap::new(),
            skip_specials,
            season_with_premium: is_premium.not().then_some(vec![]),
            season_sorting: vec![],
        }
    }

    async fn visit_series(&mut self, series: Series) -> Result<Vec<Season>> {
        // the audio locales field isn't always populated
        if !series.audio_locales.is_empty() {
            let missing_audios = missing_locales(&series.audio_locales, &self.audio_locales);
            if !missing_audios.is_empty()
                && !(self.audios_missing)(FilterMediaScope::Series(&series), missing_audios)?
            {
                return Ok(vec![]);
            }
            let missing_subtitles =
                missing_locales(&series.subtitle_locales, &self.subtitle_locales);
            if !missing_subtitles.is_empty()
                && !(self.subtitles_missing)(FilterMediaScope::Series(&series), missing_subtitles)?
            {
                return Ok(vec![]);
            }
        }

        let mut seasons = vec![];
        for season in series.seasons().await? {
            if !self.url_filter.is_season_valid(season.season_number) {
                continue;
            }
            let missing_audios = missing_locales(
                &season
                    .versions
                    .iter()
                    .map(|l| l.audio_locale.clone())
                    .collect::<Vec<Locale>>(),
                &self.audio_locales,
            );
            if !missing_audios.is_empty()
                && !(self.audios_missing)(FilterMediaScope::Season(&season), missing_audios)?
            {
                return Ok(vec![]);
            }
            seasons.push(season)
        }

        let duplicated_seasons = get_duplicated_seasons(&seasons);
        if !duplicated_seasons.is_empty() {
            if self.interactive_input {
                check_for_duplicated_seasons(&mut seasons)
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

        self.series_visited = true;

        Ok(seasons)
    }

    async fn visit_season(&mut self, season: Season) -> Result<Vec<Episode>> {
        if !self.url_filter.is_season_valid(season.season_number) {
            return Ok(vec![]);
        }

        let mut seasons = vec![];
        if self
            .audio_locales
            .iter()
            .any(|l| season.audio_locales.contains(l))
        {
            seasons.push(season.clone())
        }
        for version in season.versions {
            if season.id == version.id {
                continue;
            }
            if self.audio_locales.contains(&version.audio_locale) {
                seasons.push(version.season().await?)
            }
        }

        let mut episodes = vec![];
        for season in seasons {
            self.season_sorting.push(season.id.clone());
            let mut eps = season.episodes().await?;

            // removes any episode that does not have the audio locale of the season. yes, this is
            // the case sometimes
            if season.audio_locales.len() < 2 {
                let season_locale = season
                    .audio_locales
                    .first()
                    .cloned()
                    .unwrap_or(Locale::ja_JP);
                eps.retain(|e| e.audio_locale == season_locale)
            }

            #[allow(clippy::if_same_then_else)]
            if eps.len() < season.number_of_episodes as usize {
                if eps.is_empty()
                    && !(self.audios_missing)(
                        FilterMediaScope::Season(&season),
                        season.audio_locales.iter().collect(),
                    )?
                {
                    return Ok(vec![]);
                } else if !eps.is_empty()
                    && !(self.audios_missing)(
                        FilterMediaScope::Episode(vec![eps.first().unwrap(), eps.last().unwrap()]),
                        vec![&eps.first().unwrap().audio_locale],
                    )?
                {
                    return Ok(vec![]);
                }
            }

            episodes.extend(eps)
        }

        if self.relative_episode_number {
            for episode in &episodes {
                self.season_episodes
                    .entry(episode.season_id.clone())
                    .or_default()
                    .push(episode.clone())
            }
        }

        Ok(episodes)
    }

    async fn visit_episode(&mut self, episode: Episode) -> Result<Vec<SingleFormat>> {
        if !self
            .url_filter
            .is_episode_valid(episode.sequence_number, episode.season_number)
        {
            return Ok(vec![]);
        }

        // skip the episode if it's a special
        if self.skip_specials
            && (episode.sequence_number == 0.0 || episode.sequence_number.fract() != 0.0)
        {
            return Ok(vec![]);
        }

        let mut episodes = vec![];
        if !self.series_visited {
            if self.audio_locales.contains(&episode.audio_locale) {
                episodes.push(episode.clone())
            }
            for version in &episode.versions {
                // `episode` is also a version of itself. the if block above already adds the
                // episode if it matches the requested audio, so it doesn't need to be requested
                // here again
                if version.id == episode.id {
                    continue;
                }
                if self.audio_locales.contains(&version.audio_locale) {
                    episodes.push(version.episode().await?)
                }
            }

            let audio_locales: Vec<Locale> =
                episodes.iter().map(|e| e.audio_locale.clone()).collect();
            let missing_audios = missing_locales(&audio_locales, &self.audio_locales);
            if !missing_audios.is_empty()
                && !(self.audios_missing)(
                    FilterMediaScope::Episode(vec![&episode]),
                    missing_audios,
                )?
            {
                return Ok(vec![]);
            }

            let mut subtitle_locales: Vec<Locale> = episodes
                .iter()
                .flat_map(|e| e.subtitle_locales.clone())
                .collect();
            subtitle_locales.sort();
            subtitle_locales.dedup();
            let missing_subtitles = missing_locales(&subtitle_locales, &self.subtitle_locales);
            if !missing_subtitles.is_empty()
                && !(self.subtitles_missing)(
                    FilterMediaScope::Episode(vec![&episode]),
                    missing_subtitles,
                )?
            {
                return Ok(vec![]);
            }
        } else {
            episodes.push(episode.clone())
        }

        if let Some(seasons_with_premium) = &mut self.season_with_premium {
            let episodes_len_before = episodes.len();
            episodes.retain(|e| !e.is_premium_only && !self.is_premium);
            if episodes_len_before < episodes.len()
                && !seasons_with_premium.contains(&episode.season_number)
            {
                (self.no_premium)(episode.season_number)?;
                seasons_with_premium.push(episode.season_number)
            }

            if episodes.is_empty() {
                return Ok(vec![]);
            }
        }

        let mut relative_episode_number = None;
        let mut relative_sequence_number = None;
        if self.relative_episode_number {
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
                if ep.sequence_number != 0.0 || ep.sequence_number.fract() == 0.0 {
                    non_integer_sequence_number_count += 1
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

        Ok(episodes
            .into_iter()
            .map(|e| {
                SingleFormat::new_from_episode(
                    e.clone(),
                    e.subtitle_locales,
                    relative_episode_number.map(|n| n as u32),
                    relative_sequence_number,
                )
            })
            .collect())
    }

    async fn visit_movie_listing(&mut self, movie_listing: MovieListing) -> Result<Vec<Movie>> {
        Ok(movie_listing.movies().await?)
    }

    async fn visit_movie(&mut self, movie: Movie) -> Result<Vec<SingleFormat>> {
        Ok(vec![SingleFormat::new_from_movie(movie, vec![])])
    }

    async fn visit_music_video(&mut self, music_video: MusicVideo) -> Result<Vec<SingleFormat>> {
        Ok(vec![SingleFormat::new_from_music_video(music_video)])
    }

    async fn visit_concert(&mut self, concert: Concert) -> Result<Vec<SingleFormat>> {
        Ok(vec![SingleFormat::new_from_concert(concert)])
    }

    async fn finish(self, input: Vec<Vec<SingleFormat>>) -> Result<SingleFormatCollection> {
        let flatten_input: Vec<SingleFormat> = input.into_iter().flatten().collect();

        let mut single_format_collection = SingleFormatCollection::new();

        let mut pre_sorted: BTreeMap<String, Vec<SingleFormat>> = BTreeMap::new();
        for data in flatten_input {
            pre_sorted
                .entry(data.identifier.clone())
                .or_default()
                .push(data)
        }

        let mut sorted: Vec<(String, Vec<SingleFormat>)> = pre_sorted.into_iter().collect();
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
                self.audio_locales
                    .iter()
                    .position(|p| p == &a.audio)
                    .unwrap_or(usize::MAX)
                    .cmp(
                        &self
                            .audio_locales
                            .iter()
                            .position(|p| p == &b.audio)
                            .unwrap_or(usize::MAX),
                    )
            });
            single_format_collection.add_single_formats(data)
        }

        Ok(single_format_collection)
    }

    pub(crate) async fn visit(
        mut self,
        media_collection: MediaCollection,
    ) -> Result<SingleFormatCollection> {
        let mut items = vec![media_collection];
        let mut result = vec![];

        while !items.is_empty() {
            let mut new_items: Vec<MediaCollection> = vec![];

            for i in items {
                match i {
                    MediaCollection::Series(series) => new_items.extend(
                        self.visit_series(series)
                            .await?
                            .into_iter()
                            .map(|s| s.into())
                            .collect::<Vec<MediaCollection>>(),
                    ),
                    MediaCollection::Season(season) => new_items.extend(
                        self.visit_season(season)
                            .await?
                            .into_iter()
                            .map(|s| s.into())
                            .collect::<Vec<MediaCollection>>(),
                    ),
                    MediaCollection::Episode(episode) => {
                        result.push(self.visit_episode(episode).await?)
                    }
                    MediaCollection::MovieListing(movie_listing) => new_items.extend(
                        self.visit_movie_listing(movie_listing)
                            .await?
                            .into_iter()
                            .map(|m| m.into())
                            .collect::<Vec<MediaCollection>>(),
                    ),
                    MediaCollection::Movie(movie) => result.push(self.visit_movie(movie).await?),
                    MediaCollection::MusicVideo(music_video) => {
                        result.push(self.visit_music_video(music_video).await?)
                    }
                    MediaCollection::Concert(concert) => {
                        result.push(self.visit_concert(concert).await?)
                    }
                }
            }

            items = new_items
        }

        self.finish(result).await
    }
}

fn missing_locales<'a>(available: &[Locale], searched: &'a [Locale]) -> Vec<&'a Locale> {
    searched.iter().filter(|p| !available.contains(p)).collect()
}

/// Remove all duplicates from a [`Vec`].
pub fn real_dedup_vec<T: Clone + Eq>(input: &mut Vec<T>) {
    let mut dedup = vec![];
    for item in input.clone() {
        if !dedup.contains(&item) {
            dedup.push(item);
        }
    }
    *input = dedup
}
