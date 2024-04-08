use anyhow::{bail, Result};
use crunchyroll_rs::media::{Resolution, Stream, StreamData};
use crunchyroll_rs::Locale;

pub async fn stream_data_from_stream(
    stream: &Stream,
    resolution: &Resolution,
    subtitle: Option<Locale>,
) -> Result<Option<(StreamData, StreamData, bool)>> {
    // sometimes Crunchyroll marks episodes without real subtitles that they have subtitles and
    // reports that only hardsub episode are existing. the following lines are trying to prevent
    // potential errors which might get caused by this incorrect reporting
    // (https://github.com/crunchy-labs/crunchy-cli/issues/231)
    let mut hardsub_locales: Vec<Locale> = stream.hard_subs.keys().cloned().collect();
    let (hardsub_locale, mut contains_hardsub) = if !hardsub_locales
        .contains(&Locale::Custom("".to_string()))
        && !hardsub_locales.contains(&Locale::Custom(":".to_string()))
    {
        // if only one hardsub locale exists, assume that this stream doesn't really contains hardsubs
        if hardsub_locales.len() == 1 {
            (Some(hardsub_locales.remove(0)), false)
        } else {
            // fallback to `None`. this should trigger an error message in `stream.dash_streaming_data`
            // that the requested stream is not available
            (None, false)
        }
    } else {
        let hardsubs_requested = subtitle.is_some();
        (subtitle, hardsubs_requested)
    };

    let (mut videos, mut audios) = match stream.stream_data(hardsub_locale).await {
        Ok(data) => data,
        Err(e) => {
            // the error variant is only `crunchyroll_rs::error::Error::Input` when the requested
            // hardsub is not available
            if let crunchyroll_rs::error::Error::Input { .. } = e {
                contains_hardsub = false;
                stream.stream_data(None).await?
            } else {
                bail!(e)
            }
        }
    }
    .unwrap();
    videos.sort_by(|a, b| a.bandwidth.cmp(&b.bandwidth).reverse());
    audios.sort_by(|a, b| a.bandwidth.cmp(&b.bandwidth).reverse());

    let video_variant = match resolution.height {
        u64::MAX => Some(videos.into_iter().next().unwrap()),
        u64::MIN => Some(videos.into_iter().last().unwrap()),
        _ => videos
            .into_iter()
            .find(|v| resolution.height == v.resolution().unwrap().height),
    };
    Ok(video_variant.map(|v| (v, audios.first().unwrap().clone(), contains_hardsub)))
}
