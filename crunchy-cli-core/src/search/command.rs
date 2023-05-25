use crate::search::filter::FilterOptions;
use crate::search::format::Format;
use crate::utils::context::Context;
use crate::utils::parse::{parse_url, UrlFilter};
use crate::Execute;
use anyhow::{bail, Result};
use crunchyroll_rs::common::StreamExt;
use crunchyroll_rs::search::QueryResults;
use crunchyroll_rs::{Episode, Locale, MediaCollection, MovieListing, MusicVideo, Series};

#[derive(Debug, clap::Parser)]
pub struct Search {
    #[arg(help = "Audio languages to include")]
    #[arg(long, default_values_t = vec![crate::utils::locale::system_locale()])]
    audio: Vec<Locale>,

    #[arg(help = "Filter the locale/language of subtitles according to the value of `--audio`")]
    #[arg(long, default_value_t = false)]
    filter_subtitles: bool,

    #[arg(help = "Limit of search top search results")]
    #[arg(long, default_value_t = 5)]
    search_top_results_limit: u32,
    #[arg(help = "Limit of search series results")]
    #[arg(long, default_value_t = 0)]
    search_series_limit: u32,
    #[arg(help = "Limit of search movie listing results")]
    #[arg(long, default_value_t = 0)]
    search_movie_listing_limit: u32,
    #[arg(help = "Limit of search episode results")]
    #[arg(long, default_value_t = 0)]
    search_episode_limit: u32,
    #[arg(help = "Limit of search music results")]
    #[arg(long, default_value_t = 0)]
    search_music_limit: u32,

    /// Format of the output text.
    ///
    /// You can specify keywords in a specific pattern and they will get replaced in the output text.
    /// The required pattern for this begins with `{{`, then the keyword, and closes with `}}` (e.g. `{{episode.title}}`).
    /// For example, if you want to get the title of an episode, you can use `Title {{episode.title}}` and `{{episode.title}}` will be replaced with the episode title
    ///
    /// See the following list for all keywords and their meaning:
    ///     series.title              → Series title
    ///     series.description        → Series description
    ///
    ///     season.title              → Season title
    ///     season.description        → Season description
    ///     season.number             → Season number
    ///
    ///     episode.title             → Episode title
    ///     episode.description       → Episode description
    ///     episode.locale            → Episode locale/language
    ///     episode.number            → Episode number
    ///     episode.sequence_number   → Episode number. This number is unique unlike `episode.number` which sometimes can be duplicated
    ///
    ///     movie_listing.title       → Movie listing title
    ///     movie_listing.description → Movie listing description
    ///
    ///     movie.title               → Movie title
    ///     movie.description         → Movie description
    ///
    ///     music_video.title         → Music video title
    ///     music_video.description   → Music video description
    ///
    ///     concert.title             → Concert title
    ///     concert.description       → Concert description
    ///
    ///     stream.locale             → Stream locale/language
    ///     stream.dash_url           → Stream url in DASH format
    ///     stream.hls_url            → Stream url in HLS format
    ///
    ///     subtitle.locale           → Subtitle locale/language
    ///     subtitle.url              → Subtitle url
    #[arg(short, long, verbatim_doc_comment)]
    #[arg(default_value = "S{{season.number}}E{{episode.number}} - {{episode.title}}")]
    output: String,

    input: String,
}

#[async_trait::async_trait(?Send)]
impl Execute for Search {
    async fn execute(self, ctx: Context) -> Result<()> {
        let input = if crunchyroll_rs::parse::parse_url(&self.input).is_some() {
            match parse_url(&ctx.crunchy, self.input.clone(), true).await {
                Ok(ok) => vec![ok],
                Err(e) => bail!("url {} could not be parsed: {}", self.input, e),
            }
        } else {
            let mut output = vec![];

            let query = resolve_query(&self, ctx.crunchy.query(&self.input)).await?;
            output.extend(query.0.into_iter().map(|m| (m, UrlFilter::default())));
            output.extend(
                query
                    .1
                    .into_iter()
                    .map(|s| (s.into(), UrlFilter::default())),
            );
            output.extend(
                query
                    .2
                    .into_iter()
                    .map(|m| (m.into(), UrlFilter::default())),
            );
            output.extend(
                query
                    .3
                    .into_iter()
                    .map(|e| (e.into(), UrlFilter::default())),
            );
            output.extend(
                query
                    .4
                    .into_iter()
                    .map(|m| (m.into(), UrlFilter::default())),
            );

            output
        };

        for (media_collection, url_filter) in input {
            let filter_options = FilterOptions {
                audio: self.audio.clone(),
                filter_subtitles: self.filter_subtitles,
                url_filter,
            };

            let format = Format::new(self.output.clone(), filter_options)?;
            println!("{}", format.parse(media_collection).await?);
        }

        Ok(())
    }
}

macro_rules! resolve_query {
    ($limit:expr, $vec:expr, $item:expr) => {
        if $limit > 0 {
            let mut item_results = $item;
            while let Some(item) = item_results.next().await {
                $vec.push(item?);
                if $vec.len() >= $limit as usize {
                    break;
                }
            }
        }
    };
}

async fn resolve_query(
    search: &Search,
    query_results: QueryResults,
) -> Result<(
    Vec<MediaCollection>,
    Vec<Series>,
    Vec<MovieListing>,
    Vec<Episode>,
    Vec<MusicVideo>,
)> {
    let mut media_collection = vec![];
    let mut series = vec![];
    let mut movie_listing = vec![];
    let mut episode = vec![];
    let mut music_video = vec![];

    resolve_query!(
        search.search_top_results_limit,
        media_collection,
        query_results.top_results
    );
    resolve_query!(search.search_series_limit, series, query_results.series);
    resolve_query!(
        search.search_movie_listing_limit,
        movie_listing,
        query_results.movie_listing
    );
    resolve_query!(search.search_episode_limit, episode, query_results.episode);
    resolve_query!(search.search_music_limit, music_video, query_results.music);

    Ok((
        media_collection,
        series,
        movie_listing,
        episode,
        music_video,
    ))
}
