use crate::utils::context::Context;
use anyhow::{bail, Result};
use crunchyroll_rs::media::{Resolution, VariantData, VariantSegment};
use crunchyroll_rs::{Locale, Media, Season};
use indicatif::{ProgressBar, ProgressFinish, ProgressStyle};
use lazy_static::lazy_static;
use log::{debug, LevelFilter};
use regex::Regex;
use std::borrow::{Borrow, BorrowMut};
use std::collections::BTreeMap;
use std::io::{BufRead, Write};
use std::sync::{mpsc, Arc, Mutex};
use std::time::Duration;
use tokio::task::JoinSet;

pub fn find_resolution(
    mut streaming_data: Vec<VariantData>,
    resolution: &Resolution,
) -> Option<VariantData> {
    streaming_data.sort_by(|a, b| a.resolution.width.cmp(&b.resolution.width).reverse());
    match resolution.height {
        u64::MAX => Some(streaming_data.into_iter().next().unwrap()),
        u64::MIN => Some(streaming_data.into_iter().last().unwrap()),
        _ => streaming_data
            .into_iter()
            .find(|v| resolution.height == u64::MAX || v.resolution.height == resolution.height),
    }
}

pub async fn download_segments(
    ctx: &Context,
    writer: &mut impl Write,
    message: Option<String>,
    variant_data: VariantData,
) -> Result<()> {
    let segments = variant_data.segments().await?;
    let total_segments = segments.len();

    let client = Arc::new(ctx.crunchy.client());
    let count = Arc::new(Mutex::new(0));

    let progress = if log::max_level() == LevelFilter::Info {
        let estimated_file_size = (variant_data.bandwidth / 8)
            * segments
                .iter()
                .map(|s| s.length.unwrap_or_default().as_secs())
                .sum::<u64>();

        let progress = ProgressBar::new(estimated_file_size)
            .with_style(
                ProgressStyle::with_template(
                    ":: {msg}{bytes:>10} {bytes_per_sec:>12} [{wide_bar}] {percent:>3}%",
                )
                .unwrap()
                .progress_chars("##-"),
            )
            .with_message(message.map(|m| m + " ").unwrap_or_default())
            .with_finish(ProgressFinish::Abandon);
        Some(progress)
    } else {
        None
    };

    let cpus = num_cpus::get();
    let mut segs: Vec<Vec<VariantSegment>> = Vec::with_capacity(cpus);
    for _ in 0..cpus {
        segs.push(vec![])
    }
    for (i, segment) in segments.clone().into_iter().enumerate() {
        segs[i - ((i / cpus) * cpus)].push(segment);
    }

    let (sender, receiver) = mpsc::channel();

    let mut join_set: JoinSet<Result<()>> = JoinSet::new();
    for num in 0..cpus {
        let thread_client = client.clone();
        let thread_sender = sender.clone();
        let thread_segments = segs.remove(0);
        let thread_count = count.clone();
        join_set.spawn(async move {
            let after_download_sender = thread_sender.clone();

            // the download process is encapsulated in its own function. this is done to easily
            // catch errors which get returned with `...?` and `bail!(...)` and that the thread
            // itself can report that an error has occured
            let download = || async move {
                for (i, segment) in thread_segments.into_iter().enumerate() {
                    let mut retry_count = 0;
                    let mut buf = loop {
                        let response = thread_client
                            .get(&segment.url)
                            .timeout(Duration::from_secs(60))
                            .send()
                            .await?;

                        match response.bytes().await {
                            Ok(b) => break b.to_vec(),
                            Err(e) => {
                                if e.is_body() {
                                    if retry_count == 5 {
                                        bail!("Max retry count reached ({}), multiple errors occured while receiving segment {}: {}", retry_count, num + (i * cpus), e)
                                    }
                                    debug!("Failed to download segment {} ({}). Retrying, {} out of 5 retries left", num + (i * cpus), e, 5 - retry_count)
                                } else {
                                    bail!("{}", e)
                                }
                            }
                        }

                        retry_count += 1;
                    };

                    buf = VariantSegment::decrypt(buf.borrow_mut(), segment.key)?.to_vec();

                    let mut c = thread_count.lock().unwrap();
                    debug!(
                        "Downloaded and decrypted segment [{}/{} {:.2}%] {}",
                        num + (i * cpus),
                        total_segments,
                        ((*c + 1) as f64 / total_segments as f64) * 100f64,
                        segment.url
                    );

                    thread_sender.send((num as i32 + (i * cpus) as i32, buf))?;

                    *c += 1;
                }
                Ok(())
            };


            let result = download().await;
            if result.is_err() {
                after_download_sender.send((-1 as i32, vec![]))?;
            }

            result
        });
    }
    // drop the sender already here so it does not outlive all (download) threads which are the only
    // real consumers of it
    drop(sender);

    // this is the main loop which writes the data. it uses a BTreeMap as a buffer as the write
    // happens synchronized. the download consist of multiple segments. the map keys are representing
    // the segment number and the values the corresponding bytes
    let mut data_pos = 0;
    let mut buf: BTreeMap<i32, Vec<u8>> = BTreeMap::new();
    for (pos, bytes) in receiver.iter() {
        // if the position is lower than 0, an error occured in the sending download thread
        if pos < 0 {
            break;
        }

        if let Some(p) = &progress {
            let progress_len = p.length().unwrap();
            let estimated_segment_len = (variant_data.bandwidth / 8)
                * segments
                    .get(pos as usize)
                    .unwrap()
                    .length
                    .unwrap_or_default()
                    .as_secs();
            let bytes_len = bytes.len() as u64;

            p.set_length(progress_len - estimated_segment_len + bytes_len);
            p.inc(bytes_len)
        }

        // check if the currently sent bytes are the next in the buffer. if so, write them directly
        // to the target without first adding them to the buffer.
        // if not, add them to the buffer
        if data_pos == pos {
            writer.write_all(bytes.borrow())?;
            data_pos += 1;
        } else {
            buf.insert(pos, bytes);
        }
        // check if the buffer contains the next segment(s)
        while let Some(b) = buf.remove(&data_pos) {
            writer.write_all(b.borrow())?;
            data_pos += 1;
        }
    }

    // if any error has occured while downloading it gets returned here
    while let Some(joined) = join_set.join_next().await {
        joined??
    }

    // write the remaining buffer, if existent
    while let Some(b) = buf.remove(&data_pos) {
        writer.write_all(b.borrow())?;
        data_pos += 1;
    }

    if !buf.is_empty() {
        bail!(
            "Download buffer is not empty. Remaining segments: {}",
            buf.into_keys()
                .map(|k| k.to_string())
                .collect::<Vec<String>>()
                .join(", ")
        )
    }

    Ok(())
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum FFmpegPreset {
    Nvidia,

    Av1,
    H265,
    H264,
}

impl ToString for FFmpegPreset {
    fn to_string(&self) -> String {
        match self {
            &FFmpegPreset::Nvidia => "nvidia",
            &FFmpegPreset::Av1 => "av1",
            &FFmpegPreset::H265 => "h265",
            &FFmpegPreset::H264 => "h264",
        }
        .to_string()
    }
}

impl FFmpegPreset {
    pub(crate) fn all() -> Vec<FFmpegPreset> {
        vec![
            FFmpegPreset::Nvidia,
            FFmpegPreset::Av1,
            FFmpegPreset::H265,
            FFmpegPreset::H264,
        ]
    }

    pub(crate) fn description(self) -> String {
        match self {
            FFmpegPreset::Nvidia => "If you're have a nvidia card, use hardware / gpu accelerated video processing if available",
            FFmpegPreset::Av1 => "Encode the video(s) with the av1 codec. Hardware acceleration is currently not possible with this",
            FFmpegPreset::H265 => "Encode the video(s) with the h265 codec",
            FFmpegPreset::H264 => "Encode the video(s) with the h264 codec"
        }.to_string()
    }

    pub(crate) fn parse(s: &str) -> Result<FFmpegPreset, String> {
        Ok(match s.to_lowercase().as_str() {
            "nvidia" => FFmpegPreset::Nvidia,
            "av1" => FFmpegPreset::Av1,
            "h265" | "h.265" | "hevc" => FFmpegPreset::H265,
            "h264" | "h.264" => FFmpegPreset::H264,
            _ => return Err(format!("'{}' is not a valid ffmpeg preset", s)),
        })
    }

    pub(crate) fn ffmpeg_presets(
        mut presets: Vec<FFmpegPreset>,
    ) -> Result<(Vec<String>, Vec<String>)> {
        fn preset_check_remove(presets: &mut Vec<FFmpegPreset>, preset: FFmpegPreset) -> bool {
            if let Some(i) = presets.iter().position(|p| p == &preset) {
                presets.remove(i);
                true
            } else {
                false
            }
        }

        let nvidia = preset_check_remove(&mut presets, FFmpegPreset::Nvidia);
        if presets.len() > 1 {
            bail!(
                "Can only use one video codec, {} found: {}",
                presets.len(),
                presets
                    .iter()
                    .map(|p| p.to_string())
                    .collect::<Vec<String>>()
                    .join(", ")
            )
        }

        let (mut input, mut output) = (vec![], vec![]);
        for preset in presets {
            if nvidia {
                match preset {
                    FFmpegPreset::Av1 => bail!("'nvidia' hardware acceleration preset is not available in combination with the 'av1' codec preset"),
                    FFmpegPreset::H265 => {
                        input.extend(["-hwaccel", "cuvid", "-c:v", "h264_cuvid"]);
                        output.extend(["-c:v", "hevc_nvenc", "-c:a", "copy"]);
                    }
                    FFmpegPreset::H264 => {
                        input.extend(["-hwaccel", "cuvid", "-c:v", "h264_cuvid"]);
                        output.extend(["-c:v", "h264_nvenc", "-c:a", "copy"]);
                    }
                    _ => ()
                }
            } else {
                match preset {
                    FFmpegPreset::Av1 => {
                        output.extend(["-c:v", "libsvtav1", "-c:a", "copy"]);
                    }
                    FFmpegPreset::H265 => {
                        output.extend(["-c:v", "libx265", "-c:a", "copy"]);
                    }
                    FFmpegPreset::H264 => {
                        output.extend(["-c:v", "libx264", "-c:a", "copy"]);
                    }
                    _ => (),
                }
            }
        }

        if output.is_empty() {
            output.extend(["-c:v", "copy", "-c:a", "copy"])
        }

        Ok((
            input.into_iter().map(|i| i.to_string()).collect(),
            output.into_iter().map(|o| o.to_string()).collect(),
        ))
    }
}

lazy_static! {
    static ref DUPLICATED_SEASONS_MULTILANG_REGEX: Regex = Regex::new(r"(-arabic|-castilian|-english|-english-in|-french|-german|-hindi|-italian|-portuguese|-russian|-spanish)$").unwrap();
}

pub(crate) fn find_multiple_seasons_with_same_number(seasons: &Vec<Media<Season>>) -> Vec<u32> {
    let mut seasons_map: BTreeMap<u32, u32> = BTreeMap::new();
    for season in seasons {
        if let Some(s) = seasons_map.get_mut(&season.metadata.season_number) {
            *s += 1;
        } else {
            seasons_map.insert(season.metadata.season_number, 1);
        }
    }

    seasons_map
        .into_iter()
        .filter_map(|(k, v)| {
            if v > 1 {
                // check if the different seasons are actual the same but with different dub languages
                let mut multilang_season_vec: Vec<String> = seasons
                    .iter()
                    .map(|s| {
                        DUPLICATED_SEASONS_MULTILANG_REGEX
                            .replace(s.slug_title.trim_end_matches("-dub"), "")
                            .to_string()
                    })
                    .collect();
                multilang_season_vec.dedup();

                if multilang_season_vec.len() > 1 {
                    return Some(k);
                }
            }
            None
        })
        .collect()
}

/// Check if [`Locale::Custom("all")`] is in the provided locale list and return [`Locale::all`] if
/// so. If not, just return the provided locale list.
pub(crate) fn all_locale_in_locales(locales: Vec<Locale>) -> Vec<Locale> {
    if locales
        .iter()
        .find(|l| l.to_string().to_lowercase().trim() == "all")
        .is_some()
    {
        Locale::all()
    } else {
        locales
    }
}

pub(crate) fn interactive_season_choosing(seasons: Vec<Media<Season>>) -> Vec<Media<Season>> {
    let input_regex =
        Regex::new(r"((?P<single>\d+)|(?P<range_from>\d+)-(?P<range_to>\d+)?)(\s|$)").unwrap();

    let mut seasons_map: BTreeMap<u32, Vec<Media<Season>>> = BTreeMap::new();
    for season in seasons {
        if let Some(s) = seasons_map.get_mut(&season.metadata.season_number) {
            s.push(season);
        } else {
            seasons_map.insert(season.metadata.season_number, vec![season]);
        }
    }

    for (num, season_vec) in seasons_map.iter_mut() {
        if season_vec.len() == 1 {
            continue;
        }

        // check if the different seasons are actual the same but with different dub languages
        let mut multilang_season_vec: Vec<String> = season_vec
            .iter()
            .map(|s| {
                DUPLICATED_SEASONS_MULTILANG_REGEX
                    .replace(s.slug_title.trim_end_matches("-dub"), "")
                    .to_string()
            })
            .collect();
        multilang_season_vec.dedup();

        if multilang_season_vec.len() == 1 {
            continue;
        }

        println!(":: Found multiple seasons for season number {}", num);
        println!(":: Select the number of the seasons you want to download (eg \"1 2 4\", \"1-3\", \"1-3 5\"):");
        for (i, season) in season_vec.iter().enumerate() {
            println!(":: \t{}. {}", i + 1, season.title)
        }
        let mut stdout = std::io::stdout();
        let _ = write!(stdout, ":: => ");
        let _ = stdout.flush();
        let mut user_input = String::new();
        std::io::stdin()
            .lock()
            .read_line(&mut user_input)
            .expect("cannot open stdin");

        let mut nums = vec![];
        for capture in input_regex.captures_iter(&user_input) {
            if let Some(single) = capture.name("single") {
                nums.push(single.as_str().parse().unwrap());
            } else {
                let range_from = capture.name("range_from");
                let range_to = capture.name("range_to");

                // input is '-' which means use all seasons
                if range_from.is_none() && range_to.is_none() {
                    nums = vec![];
                    break;
                }
                let from = range_from
                    .map(|f| f.as_str().parse::<usize>().unwrap() - 1)
                    .unwrap_or(usize::MIN);
                let to = range_from
                    .map(|f| f.as_str().parse::<usize>().unwrap() - 1)
                    .unwrap_or(usize::MAX);

                nums.extend(
                    season_vec
                        .iter()
                        .enumerate()
                        .filter_map(|(i, _)| if i >= from && i <= to { Some(i) } else { None })
                        .collect::<Vec<usize>>(),
                )
            }
        }
        nums.dedup();

        if !nums.is_empty() {
            let mut remove_count = 0;
            for i in 0..season_vec.len() - 1 {
                if !nums.contains(&i) {
                    season_vec.remove(i - remove_count);
                    remove_count += 1
                }
            }
        }
    }

    seasons_map
        .into_values()
        .into_iter()
        .flatten()
        .collect::<Vec<Media<Season>>>()
}
