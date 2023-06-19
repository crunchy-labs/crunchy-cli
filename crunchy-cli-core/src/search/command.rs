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
#[clap(about = "Search in videos")]
#[command(arg_required_else_help(true))]
pub struct Search {
    #[arg(help = format!("Audio languages to include. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Audio languages to include. \
    Available languages are:\n  {}", Locale::all().into_iter().map(|l| format!("{:<6} → {}", l.to_string(), l.to_human_readable())).collect::<Vec<String>>().join("\n  ")))]
    #[arg(long, default_values_t = vec![crate::utils::locale::system_locale()])]
    audio: Vec<Locale>,

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
    ///     series.id                 → Series id
    ///     series.title              → Series title
    ///     series.description        → Series description
    ///     series.release_year       → Series release year
    ///
    ///     season.id                 → Season id
    ///     season.title              → Season title
    ///     season.description        → Season description
    ///     season.number             → Season number
    ///     season.episodes           → Number of episodes the season has
    ///
    ///     episode.id                → Episode id
    ///     episode.title             → Episode title
    ///     episode.description       → Episode description
    ///     episode.locale            → Episode locale/language
    ///     episode.number            → Episode number
    ///     episode.sequence_number   → Episode number. This number is unique unlike `episode.number` which sometimes can be duplicated
    ///     episode.duration          → Episode duration in milliseconds
    ///     episode.air_date          → Episode air date as unix timestamp
    ///     episode.premium_only      → If the episode is only available with Crunchyroll premium
    ///
    ///     movie_listing.id          → Movie listing id
    ///     movie_listing.title       → Movie listing title
    ///     movie_listing.description → Movie listing description
    ///
    ///     movie.id                  → Movie id
    ///     movie.title               → Movie title
    ///     movie.description         → Movie description
    ///     movie.duration            → Movie duration in milliseconds
    ///     movie.premium_only        → If the movie is only available with Crunchyroll premium
    ///
    ///     music_video.id            → Music video id
    ///     music_video.title         → Music video title
    ///     music_video.description   → Music video description
    ///     music_video.duration      → Music video duration in milliseconds
    ///     music_video.premium_only  → If the music video is only available with Crunchyroll premium
    ///
    ///     concert.id                → Concert id
    ///     concert.title             → Concert title
    ///     concert.description       → Concert description
    ///     concert.duration          → Concert duration in milliseconds
    ///     concert.premium_only      → If the concert is only available with Crunchyroll premium
    ///
    ///     stream.locale             → Stream locale/language
    ///     stream.dash_url           → Stream url in DASH format
    ///     stream.hls_url            → Stream url in HLS format
    ///
    ///     subtitle.locale           → Subtitle locale/language
    ///     subtitle.url              → Url to the subtitle
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
