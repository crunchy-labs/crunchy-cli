use anyhow::{bail, Result};
use crunchyroll_rs::media::{Resolution, Stream, StreamData};
use crunchyroll_rs::Locale;

pub async fn stream_data_from_stream(
    stream: &Stream,
    resolution: &Resolution,
    hardsub_subtitle: Option<Locale>,
) -> Result<Option<(StreamData, StreamData, bool)>> {
    let (hardsub_locale, mut contains_hardsub) = if hardsub_subtitle.is_some() {
        (hardsub_subtitle, true)
    } else {
        (None, false)
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

    if videos.iter().any(|v| v.drm.is_some()) || audios.iter().any(|v| v.drm.is_some()) {
        bail!("Stream is DRM protected")
    }

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
