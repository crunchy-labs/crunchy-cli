use anyhow::{anyhow, bail, Result};
use crunchyroll_rs::media::Resolution;
use crunchyroll_rs::{Crunchyroll, MediaCollection, UrlType};
use log::debug;
use regex::Regex;

/// Define a find, based on season and episode number to find episodes / movies.
/// If a struct instance equals the [`Default::default()`] it's considered that no find is applied.
/// If `from_*` is [`None`] they're set to [`u32::MIN`].
/// If `to_*` is [`None`] they're set to [`u32::MAX`].
#[derive(Debug, Default)]
pub struct InnerUrlFilter {
    from_episode: Option<f32>,
    to_episode: Option<f32>,
    from_season: Option<u32>,
    to_season: Option<u32>,
}

#[derive(Debug)]
pub struct UrlFilter {
    inner: Vec<InnerUrlFilter>,
}

impl Default for UrlFilter {
    fn default() -> Self {
        Self {
            inner: vec![InnerUrlFilter::default()],
        }
    }
}

impl UrlFilter {
    pub fn is_season_valid(&self, season: u32) -> bool {
        self.inner.iter().any(|f| {
            let from_season = f.from_season.unwrap_or(u32::MIN);
            let to_season = f.to_season.unwrap_or(u32::MAX);

            season >= from_season && season <= to_season
        })
    }

    pub fn is_episode_valid(&self, episode: f32, season: u32) -> bool {
        self.inner.iter().any(|f| {
            let from_episode = f.from_episode.unwrap_or(f32::MIN);
            let to_episode = f.to_episode.unwrap_or(f32::MAX);
            let from_season = f.from_season.unwrap_or(u32::MIN);
            let to_season = f.to_season.unwrap_or(u32::MAX);

            episode >= from_episode
                && episode <= to_episode
                && season >= from_season
                && season <= to_season
        })
    }
}

/// Parse a url and return all [`crunchyroll_rs::Media<crunchyroll_rs::Episode>`] &
/// [`crunchyroll_rs::Media<crunchyroll_rs::Movie>`] which could be related to it.
///
/// The `with_filter` arguments says if filtering should be enabled for the url. Filtering is a
/// specific pattern at the end of the url which declares which parts of the url content should be
/// returned / filtered (out). _This only works if the url points to a series_.
///
/// Examples how filtering works:
/// - `...[E5]` - Download the fifth episode.
/// - `...[S1]` - Download the full first season.
/// - `...[-S2]` - Download all seasons up to and including season 2.
/// - `...[S3E4-]` - Download all episodes from and including season 3, episode 4.
/// - `...[S1E4-S3]` - Download all episodes from and including season 1, episode 4, until andincluding season 3.
/// - `...[S3,S5]` - Download episode 3 and 5.
/// - `...[S1-S3,S4E2-S4E6]` - Download season 1 to 3 and episode 2 to episode 6 of season 4.

/// In practice, it would look like this: `https://crunchyroll.com/series/12345678/example[S1E5-S3E2]`.
pub async fn parse_url(
    crunchy: &Crunchyroll,
    mut url: String,
    with_filter: bool,
) -> Result<(MediaCollection, UrlFilter)> {
    let url_filter = if with_filter {
        debug!("Url may contain filters");

        let open_index = url.rfind('[').unwrap_or(0);
        let close_index = url.rfind(']').unwrap_or(0);

        let filter = if open_index < close_index {
            let filter = url.as_str()[open_index + 1..close_index].to_string();
            url = url.as_str()[0..open_index].to_string();
            filter
        } else {
            "".to_string()
        };

        let filter_regex = Regex::new(r"((S(?P<from_season>\d+))?(E(?P<from_episode>\d+))?)(((?P<dash>-)((S(?P<to_season>\d+))?(E(?P<to_episode>\d+))?))?)(,|$)").unwrap();

        let mut filters = vec![];

        for capture in filter_regex.captures_iter(&filter) {
            let dash = capture.name("dash").is_some();
            let from_episode = capture
                .name("from_episode")
                .map_or(anyhow::Ok(None), |fe| Ok(Some(fe.as_str().parse()?)))?;
            let to_episode = capture
                .name("to_episode")
                .map_or(anyhow::Ok(if dash { None } else { from_episode }), |te| {
                    Ok(Some(te.as_str().parse()?))
                })?;
            let from_season = capture
                .name("from_season")
                .map_or(anyhow::Ok(None), |fs| Ok(Some(fs.as_str().parse()?)))?;
            let to_season = capture
                .name("to_season")
                .map_or(anyhow::Ok(if dash { None } else { from_season }), |ts| {
                    Ok(Some(ts.as_str().parse()?))
                })?;

            filters.push(InnerUrlFilter {
                from_episode,
                to_episode,
                from_season,
                to_season,
            })
        }

        let url_filter = UrlFilter { inner: filters };

        debug!("Url find: {:?}", url_filter);

        url_filter
    } else {
        UrlFilter::default()
    };

    // check if the url is the old series/episode scheme which still occurs in some places (like the
    // rss)
    let old_url_regex = Regex::new(r"https?://(www\.)?crunchyroll\.com/.+").unwrap();
    if old_url_regex.is_match(&url) {
        debug!("Detected maybe old url");
        // replace the 'http' prefix with 'https' as https is not supported by the reqwest client
        if url.starts_with("http://") {
            url.replace_range(0..4, "https")
        }
        // the old url redirects to the new url. request the old url, follow the redirects and
        // extract the final url
        url = crunchy.client().get(&url).send().await?.url().to_string()
    }

    let parsed_url = crunchyroll_rs::parse_url(url).map_or(Err(anyhow!("Invalid url")), Ok)?;
    debug!("Url type: {:?}", parsed_url);
    let media_collection = match parsed_url {
        UrlType::Series(id)
        | UrlType::MovieListing(id)
        | UrlType::EpisodeOrMovie(id)
        | UrlType::MusicVideo(id)
        | UrlType::Concert(id) => crunchy.media_collection_from_id(id).await?,
    };

    Ok((media_collection, url_filter))
}

/// Parse a resolution given as a [`String`] to a [`crunchyroll_rs::media::Resolution`].
pub fn parse_resolution(mut resolution: String) -> Result<Resolution> {
    resolution = resolution.to_lowercase();

    if resolution == "best" {
        Ok(Resolution {
            width: u64::MAX,
            height: u64::MAX,
        })
    } else if resolution == "worst" {
        Ok(Resolution {
            width: u64::MIN,
            height: u64::MIN,
        })
    } else if resolution.ends_with('p') {
        let without_p = resolution.as_str()[0..resolution.len() - 1]
            .parse()
            .map_err(|_| anyhow!("Could not find resolution"))?;
        Ok(Resolution {
            width: without_p * 16 / 9,
            height: without_p,
        })
    } else if let Some((w, h)) = resolution.split_once('x') {
        Ok(Resolution {
            width: w
                .parse()
                .map_err(|_| anyhow!("Could not find resolution"))?,
            height: h
                .parse()
                .map_err(|_| anyhow!("Could not find resolution"))?,
        })
    } else {
        bail!("Could not find resolution")
    }
}

/// Dirty implementation of [`f32::fract`] with more accuracy.
pub fn fract(input: f32) -> f32 {
    if input.fract() == 0.0 {
        return 0.0;
    }
    format!("0.{}", input.to_string().split('.').last().unwrap())
        .parse::<f32>()
        .unwrap()
}
