use anyhow::Result;
use crunchyroll_rs::media::{Resolution, Stream, VariantData};

pub async fn variant_data_from_stream(
    stream: &Stream,
    resolution: &Resolution,
) -> Result<Option<(VariantData, VariantData)>> {
    let mut streaming_data = stream.dash_streaming_data(None).await?;
    streaming_data
        .0
        .sort_by(|a, b| a.bandwidth.cmp(&b.bandwidth).reverse());
    streaming_data
        .1
        .sort_by(|a, b| a.bandwidth.cmp(&b.bandwidth).reverse());

    let video_variant = match resolution.height {
        u64::MAX => Some(streaming_data.0.into_iter().next().unwrap()),
        u64::MIN => Some(streaming_data.0.into_iter().last().unwrap()),
        _ => streaming_data
            .0
            .into_iter()
            .find(|v| resolution.height == v.resolution.height),
    };
    Ok(video_variant.map(|v| (v, streaming_data.1.first().unwrap().clone())))
}
