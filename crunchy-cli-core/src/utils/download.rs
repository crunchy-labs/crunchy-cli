use crate::utils::context::Context;
use crate::utils::ffmpeg::FFmpegPreset;
use crate::utils::log::progress;
use crate::utils::os::{is_special_file, temp_directory, tempfile};
use anyhow::{bail, Result};
use chrono::NaiveTime;
use crunchyroll_rs::media::{Subtitle, VariantData, VariantSegment};
use crunchyroll_rs::Locale;
use indicatif::{ProgressBar, ProgressFinish, ProgressStyle};
use log::{debug, warn, LevelFilter};
use regex::Regex;
use std::borrow::Borrow;
use std::borrow::BorrowMut;
use std::cmp::Ordering;
use std::collections::BTreeMap;
use std::env;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::{mpsc, Arc, Mutex};
use std::time::Duration;
use tempfile::TempPath;
use tokio::task::JoinSet;

#[derive(Clone, Debug)]
pub enum MergeBehavior {
    Video,
    Audio,
    Auto,
}

impl MergeBehavior {
    pub fn parse(s: &str) -> Result<MergeBehavior, String> {
        Ok(match s.to_lowercase().as_str() {
            "video" => MergeBehavior::Video,
            "audio" => MergeBehavior::Audio,
            "auto" => MergeBehavior::Auto,
            _ => return Err(format!("'{}' is not a valid merge behavior", s)),
        })
    }
}

#[derive(Clone, derive_setters::Setters)]
pub struct DownloadBuilder {
    ffmpeg_preset: FFmpegPreset,
    default_subtitle: Option<Locale>,
    output_format: Option<String>,
    audio_sort: Option<Vec<Locale>>,
    subtitle_sort: Option<Vec<Locale>>,
}

impl DownloadBuilder {
    pub fn new() -> DownloadBuilder {
        Self {
            ffmpeg_preset: FFmpegPreset::default(),
            default_subtitle: None,
            output_format: None,
            audio_sort: None,
            subtitle_sort: None,
        }
    }

    pub fn build(self) -> Downloader {
        Downloader {
            ffmpeg_preset: self.ffmpeg_preset,
            default_subtitle: self.default_subtitle,
            output_format: self.output_format,
            audio_sort: self.audio_sort,
            subtitle_sort: self.subtitle_sort,

            formats: vec![],
        }
    }
}

struct FFmpegMeta {
    path: TempPath,
    language: Locale,
    title: String,
}

pub struct DownloadFormat {
    pub video: (VariantData, Locale),
    pub audios: Vec<(VariantData, Locale)>,
    pub subtitles: Vec<(Subtitle, bool)>,
}

pub struct Downloader {
    ffmpeg_preset: FFmpegPreset,
    default_subtitle: Option<Locale>,
    output_format: Option<String>,
    audio_sort: Option<Vec<Locale>>,
    subtitle_sort: Option<Vec<Locale>>,

    formats: Vec<DownloadFormat>,
}

impl Downloader {
    pub fn add_format(&mut self, format: DownloadFormat) {
        self.formats.push(format);
    }

    pub async fn download(mut self, ctx: &Context, dst: &Path) -> Result<()> {
        // `.unwrap_or_default()` here unless https://doc.rust-lang.org/stable/std/path/fn.absolute.html
        // gets stabilized as the function might throw error on weird file paths
        let required = self.check_free_space(dst).await.unwrap_or_default();
        if let Some((path, tmp_required)) = &required.0 {
            let kb = (*tmp_required as f64) / 1024.0;
            let mb = kb / 1024.0;
            let gb = mb / 1024.0;
            warn!(
                "You may have not enough disk space to store temporary files. The temp directory ({}) should have at least {}{} free space",
                path.to_string_lossy(),
                if gb < 1.0 { mb.ceil().to_string() } else { format!("{:.2}", gb) },
                if gb < 1.0 { "MB" } else { "GB" }
            )
        }
        if let Some((path, dst_required)) = &required.1 {
            let kb = (*dst_required as f64) / 1024.0;
            let mb = kb / 1024.0;
            let gb = mb / 1024.0;
            warn!(
                "You may have not enough disk space to store the output file. The directory {} should have at least {}{} free space",
                path.to_string_lossy(),
                if gb < 1.0 { mb.ceil().to_string() } else { format!("{:.2}", gb) },
                if gb < 1.0 { "MB" } else { "GB" }
            )
        }

        if let Some(audio_sort_locales) = &self.audio_sort {
            self.formats.sort_by(|a, b| {
                audio_sort_locales
                    .iter()
                    .position(|l| l == &a.video.1)
                    .cmp(&audio_sort_locales.iter().position(|l| l == &b.video.1))
            });
        }
        for format in self.formats.iter_mut() {
            if let Some(audio_sort_locales) = &self.audio_sort {
                format.audios.sort_by(|(_, a), (_, b)| {
                    audio_sort_locales
                        .iter()
                        .position(|l| l == a)
                        .cmp(&audio_sort_locales.iter().position(|l| l == b))
                })
            }
            if let Some(subtitle_sort) = &self.subtitle_sort {
                format
                    .subtitles
                    .sort_by(|(a_subtitle, a_not_cc), (b_subtitle, b_not_cc)| {
                        let ordering = subtitle_sort
                            .iter()
                            .position(|l| l == &a_subtitle.locale)
                            .cmp(&subtitle_sort.iter().position(|l| l == &b_subtitle.locale));
                        if matches!(ordering, Ordering::Equal) {
                            a_not_cc.cmp(b_not_cc).reverse()
                        } else {
                            ordering
                        }
                    })
            }
        }

        let mut videos = vec![];
        let mut audios = vec![];
        let mut subtitles = vec![];

        for (i, format) in self.formats.iter().enumerate() {
            let fmt_space = format
                .audios
                .iter()
                .map(|(_, locale)| format!("Downloading {} audio", locale).len())
                .max()
                .unwrap();

            let video_path = self
                .download_video(
                    ctx,
                    &format.video.0,
                    format!("{:<1$}", format!("Downloading video #{}", i + 1), fmt_space),
                )
                .await?;
            for (variant_data, locale) in format.audios.iter() {
                let audio_path = self
                    .download_audio(
                        ctx,
                        variant_data,
                        format!("{:<1$}", format!("Downloading {} audio", locale), fmt_space),
                    )
                    .await?;
                audios.push(FFmpegMeta {
                    path: audio_path,
                    language: locale.clone(),
                    title: if i == 0 {
                        locale.to_human_readable()
                    } else {
                        format!("{} [Video: #{}]", locale.to_human_readable(), i + 1)
                    },
                })
            }
            let len = get_video_length(&video_path)?;
            for (subtitle, not_cc) in format.subtitles.iter() {
                let subtitle_path = self.download_subtitle(subtitle.clone(), len).await?;
                let mut subtitle_title = subtitle.locale.to_human_readable();
                if !not_cc {
                    subtitle_title += " (CC)"
                }
                if i != 0 {
                    subtitle_title += &format!(" [Video: #{}]", i + 1)
                }
                subtitles.push(FFmpegMeta {
                    path: subtitle_path,
                    language: subtitle.locale.clone(),
                    title: subtitle_title,
                })
            }
            videos.push(FFmpegMeta {
                path: video_path,
                language: format.video.1.clone(),
                title: if self.formats.len() == 1 {
                    "Default".to_string()
                } else {
                    format!("#{}", i + 1)
                },
            });
        }

        let mut input = vec![];
        let mut maps = vec![];
        let mut metadata = vec![];

        for (i, meta) in videos.iter().enumerate() {
            input.extend(["-i".to_string(), meta.path.to_string_lossy().to_string()]);
            maps.extend(["-map".to_string(), i.to_string()]);
            metadata.extend([
                format!("-metadata:s:v:{}", i),
                format!("title={}", meta.title),
            ]);
            // the empty language metadata is created to avoid that metadata from the original track
            // is copied
            metadata.extend([format!("-metadata:s:v:{}", i), format!("language=")])
        }
        for (i, meta) in audios.iter().enumerate() {
            input.extend(["-i".to_string(), meta.path.to_string_lossy().to_string()]);
            maps.extend(["-map".to_string(), (i + videos.len()).to_string()]);
            metadata.extend([
                format!("-metadata:s:a:{}", i),
                format!("language={}", meta.language),
            ]);
            metadata.extend([
                format!("-metadata:s:a:{}", i),
                format!("title={}", meta.title),
            ]);
        }

        // this formats are supporting embedding subtitles into the video container instead of
        // burning it into the video stream directly
        let container_supports_softsubs =
            ["mkv", "mp4"].contains(&dst.extension().unwrap_or_default().to_str().unwrap());

        if container_supports_softsubs {
            for (i, meta) in subtitles.iter().enumerate() {
                input.extend(["-i".to_string(), meta.path.to_string_lossy().to_string()]);
                maps.extend([
                    "-map".to_string(),
                    (i + videos.len() + audios.len()).to_string(),
                ]);
                metadata.extend([
                    format!("-metadata:s:s:{}", i),
                    format!("language={}", meta.language),
                ]);
                metadata.extend([
                    format!("-metadata:s:s:{}", i),
                    format!("title={}", meta.title),
                ]);
            }
        }

        let (input_presets, mut output_presets) = self.ffmpeg_preset.into_input_output_args();

        let mut command_args = vec!["-y".to_string(), "-hide_banner".to_string()];
        command_args.extend(input_presets);
        command_args.extend(input);
        command_args.extend(maps);
        command_args.extend(metadata);

        // set default subtitle
        if let Some(default_subtitle) = self.default_subtitle {
            if let Some(position) = subtitles
                .iter()
                .position(|m| m.language == default_subtitle)
            {
                match dst.extension().unwrap_or_default().to_str().unwrap() {
                    "mkv" => (),
                    "mp4" => output_presets.extend([
                        "-movflags".to_string(),
                        "faststart".to_string(),
                        "-c:s".to_string(),
                        "mov_text".to_string(),
                    ]),
                    _ => {
                        // remove '-c:v copy' and '-c:a copy' from output presets as its causes issues with
                        // burning subs into the video
                        let mut last = String::new();
                        let mut remove_count = 0;
                        for (i, s) in output_presets.clone().iter().enumerate() {
                            if (last == "-c:v" || last == "-c:a") && s == "copy" {
                                // remove last
                                output_presets.remove(i - remove_count - 1);
                                remove_count += 1;
                                output_presets.remove(i - remove_count);
                                remove_count += 1;
                            }
                            last = s.clone();
                        }

                        output_presets.extend([
                            "-vf".to_string(),
                            format!(
                                "ass={}",
                                subtitles.get(position).unwrap().path.to_str().unwrap()
                            ),
                        ])
                    }
                }
            }

            if container_supports_softsubs {
                if let Some(position) = subtitles
                    .iter()
                    .position(|meta| meta.language == default_subtitle)
                {
                    command_args.extend([
                        format!("-disposition:s:s:{}", position),
                        "forced".to_string(),
                    ])
                }
            }
        }

        command_args.extend(output_presets);
        if let Some(output_format) = self.output_format {
            command_args.extend(["-f".to_string(), output_format]);
        }
        command_args.push(dst.to_str().unwrap().to_string());

        debug!("ffmpeg {}", command_args.join(" "));

        // create parent directory if it does not exist
        if let Some(parent) = dst.parent() {
            if !parent.exists() {
                std::fs::create_dir_all(parent)?
            }
        }

        let progress_handler = progress!("Generating output file");

        let ffmpeg = Command::new("ffmpeg")
            // pass ffmpeg stdout to real stdout only if output file is stdout
            .stdout(if dst.to_str().unwrap() == "-" {
                Stdio::inherit()
            } else {
                Stdio::null()
            })
            .stderr(Stdio::piped())
            .args(command_args)
            .output()?;
        if !ffmpeg.status.success() {
            bail!("{}", String::from_utf8_lossy(ffmpeg.stderr.as_slice()))
        }

        progress_handler.stop("Output file generated");

        Ok(())
    }

    async fn check_free_space(
        &self,
        dst: &Path,
    ) -> Result<(Option<(PathBuf, u64)>, Option<(PathBuf, u64)>)> {
        let mut all_variant_data = vec![];
        for format in &self.formats {
            all_variant_data.push(&format.video.0);
            all_variant_data.extend(format.audios.iter().map(|(a, _)| a))
        }
        let mut estimated_required_space: u64 = 0;
        for variant_data in all_variant_data {
            // nearly no overhead should be generated with this call(s) as we're using dash as
            // stream provider and generating the dash segments does not need any fetching of
            // additional (http) resources as hls segments would
            let segments = variant_data.segments().await?;

            // sum the length of all streams up
            estimated_required_space += estimate_variant_file_size(variant_data, &segments);
        }

        let tmp_stat = fs2::statvfs(temp_directory()).unwrap();
        let mut dst_file = if dst.is_absolute() {
            dst.to_path_buf()
        } else {
            env::current_dir()?.join(dst)
        };
        for ancestor in dst_file.ancestors() {
            if ancestor.exists() {
                dst_file = ancestor.to_path_buf();
                break;
            }
        }
        let dst_stat = fs2::statvfs(&dst_file).unwrap();

        let mut tmp_space = tmp_stat.available_space();
        let mut dst_space = dst_stat.available_space();

        // this checks if the partition the two directories are located on are the same to prevent
        // that the space fits both file sizes each but not together. this is done by checking the
        // total space if each partition and the free space of each partition (the free space can
        // differ by 10MB as some tiny I/O operations could be performed between the two calls which
        // are checking the disk space)
        if tmp_stat.total_space() == dst_stat.total_space()
            && (tmp_stat.available_space() as i64 - dst_stat.available_space() as i64).abs() < 10240
        {
            tmp_space *= 2;
            dst_space *= 2;
        }

        let mut tmp_required = None;
        let mut dst_required = None;

        if tmp_space < estimated_required_space {
            tmp_required = Some((temp_directory(), estimated_required_space))
        }
        if (!is_special_file(dst) && dst.to_string_lossy() != "-")
            && dst_space < estimated_required_space
        {
            dst_required = Some((dst_file, estimated_required_space))
        }
        Ok((tmp_required, dst_required))
    }

    async fn download_video(
        &self,
        ctx: &Context,
        variant_data: &VariantData,
        message: String,
    ) -> Result<TempPath> {
        let tempfile = tempfile(".mp4")?;
        let (mut file, path) = tempfile.into_parts();

        download_segments(ctx, &mut file, Some(message), variant_data).await?;

        Ok(path)
    }

    async fn download_audio(
        &self,
        ctx: &Context,
        variant_data: &VariantData,
        message: String,
    ) -> Result<TempPath> {
        let tempfile = tempfile(".m4a")?;
        let (mut file, path) = tempfile.into_parts();

        download_segments(ctx, &mut file, Some(message), variant_data).await?;

        Ok(path)
    }

    async fn download_subtitle(
        &self,
        subtitle: Subtitle,
        max_length: NaiveTime,
    ) -> Result<TempPath> {
        let tempfile = tempfile(".ass")?;
        let (mut file, path) = tempfile.into_parts();

        let mut buf = vec![];
        subtitle.write_to(&mut buf).await?;
        fix_subtitle_look_and_feel(&mut buf);
        fix_subtitle_length(&mut buf, max_length);

        file.write_all(buf.as_slice())?;

        Ok(path)
    }
}

pub async fn download_segments(
    ctx: &Context,
    writer: &mut impl Write,
    message: Option<String>,
    variant_data: &VariantData,
) -> Result<()> {
    let segments = variant_data.segments().await?;
    let total_segments = segments.len();

    let client = Arc::new(ctx.crunchy.client());
    let count = Arc::new(Mutex::new(0));

    let progress = if log::max_level() == LevelFilter::Info {
        let estimated_file_size = estimate_variant_file_size(variant_data, &segments);

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
            // itself can report that an error has occurred
            let download = || async move {
                for (i, segment) in thread_segments.into_iter().enumerate() {
                    let mut retry_count = 0;
                    let mut buf = loop {
                        let request = thread_client
                            .get(&segment.url)
                            .timeout(Duration::from_secs(60))
                            .send();

                        let response = match request.await {
                            Ok(r) => r,
                            Err(e) => {
                                if retry_count == 5 {
                                    bail!("Max retry count reached ({}), multiple errors occurred while receiving segment {}: {}", retry_count, num + (i * cpus), e)
                                }
                                debug!("Failed to download segment {} ({}). Retrying, {} out of 5 retries left", num + (i * cpus), e, 5 - retry_count);
                                continue
                            }
                        };

                        match response.bytes().await {
                            Ok(b) => break b.to_vec(),
                            Err(e) => {
                                if e.is_body() {
                                    if retry_count == 5 {
                                        bail!("Max retry count reached ({}), multiple errors occurred while receiving segment {}: {}", retry_count, num + (i * cpus), e)
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
                        num + (i * cpus) + 1,
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
    // drop the sender already here so it does not outlive all download threads which are the only
    // real consumers of it
    drop(sender);

    // this is the main loop which writes the data. it uses a BTreeMap as a buffer as the write
    // happens synchronized. the download consist of multiple segments. the map keys are representing
    // the segment number and the values the corresponding bytes
    let mut data_pos = 0;
    let mut buf: BTreeMap<i32, Vec<u8>> = BTreeMap::new();
    for (pos, bytes) in receiver.iter() {
        // if the position is lower than 0, an error occurred in the sending download thread
        if pos < 0 {
            break;
        }

        if let Some(p) = &progress {
            let progress_len = p.length().unwrap();
            let estimated_segment_len =
                (variant_data.bandwidth / 8) * segments.get(pos as usize).unwrap().length.as_secs();
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

    // if any error has occurred while downloading it gets returned here
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

fn estimate_variant_file_size(variant_data: &VariantData, segments: &Vec<VariantSegment>) -> u64 {
    (variant_data.bandwidth / 8) * segments.iter().map(|s| s.length.as_secs()).sum::<u64>()
}

/// Add `ScaledBorderAndShadows: yes` to subtitles; without it they look very messy on some video
/// players. See [crunchy-labs/crunchy-cli#66](https://github.com/crunchy-labs/crunchy-cli/issues/66)
/// for more information.
fn fix_subtitle_look_and_feel(raw: &mut Vec<u8>) {
    let mut script_info = false;
    let mut new = String::new();

    for line in String::from_utf8_lossy(raw.as_slice()).split('\n') {
        if line.trim().starts_with('[') && script_info {
            new.push_str("ScaledBorderAndShadow: yes\n");
            script_info = false
        } else if line.trim() == "[Script Info]" {
            script_info = true
        }
        new.push_str(line);
        new.push('\n')
    }

    *raw = new.into_bytes()
}

/// Get the length of a video. This is required because sometimes subtitles have an unnecessary entry
/// long after the actual video ends with artificially extends the video length on some video players.
/// To prevent this, the video length must be hard set. See
/// [crunchy-labs/crunchy-cli#32](https://github.com/crunchy-labs/crunchy-cli/issues/32) for more
/// information.
pub fn get_video_length(path: &Path) -> Result<NaiveTime> {
    let video_length = Regex::new(r"Duration:\s(?P<time>\d+:\d+:\d+\.\d+),")?;

    let ffmpeg = Command::new("ffmpeg")
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .arg("-y")
        .arg("-hide_banner")
        .args(["-i", path.to_str().unwrap()])
        .output()?;
    let ffmpeg_output = String::from_utf8(ffmpeg.stderr)?;
    let caps = video_length.captures(ffmpeg_output.as_str()).unwrap();

    Ok(NaiveTime::parse_from_str(caps.name("time").unwrap().as_str(), "%H:%M:%S%.f").unwrap())
}

/// Fix the length of subtitles to a specified maximum amount. This is required because sometimes
/// subtitles have an unnecessary entry long after the actual video ends with artificially extends
/// the video length on some video players. To prevent this, the video length must be hard set. See
/// [crunchy-labs/crunchy-cli#32](https://github.com/crunchy-labs/crunchy-cli/issues/32) for more
/// information.
fn fix_subtitle_length(raw: &mut Vec<u8>, max_length: NaiveTime) {
    let re =
        Regex::new(r#"^Dialogue:\s\d+,(?P<start>\d+:\d+:\d+\.\d+),(?P<end>\d+:\d+:\d+\.\d+),"#)
            .unwrap();

    // chrono panics if we try to format NaiveTime with `%2f` and the nano seconds has more than 2
    // digits so them have to be reduced manually to avoid the panic
    fn format_naive_time(native_time: NaiveTime) -> String {
        let formatted_time = native_time.format("%f").to_string();
        format!(
            "{}.{}",
            native_time.format("%T"),
            if formatted_time.len() <= 2 {
                native_time.format("%2f").to_string()
            } else {
                formatted_time.split_at(2).0.parse().unwrap()
            }
        )
    }

    let length_as_string = format_naive_time(max_length);
    let mut new = String::new();

    for line in String::from_utf8_lossy(raw.as_slice()).split('\n') {
        if let Some(capture) = re.captures(line) {
            let start = capture.name("start").map_or(NaiveTime::default(), |s| {
                NaiveTime::parse_from_str(s.as_str(), "%H:%M:%S.%f").unwrap()
            });
            let end = capture.name("end").map_or(NaiveTime::default(), |s| {
                NaiveTime::parse_from_str(s.as_str(), "%H:%M:%S.%f").unwrap()
            });

            if start > max_length {
                continue;
            } else if end > max_length {
                new.push_str(
                    re.replace(
                        line,
                        format!(
                            "Dialogue: {},{},",
                            format_naive_time(start),
                            &length_as_string
                        ),
                    )
                    .to_string()
                    .as_str(),
                )
            } else {
                new.push_str(line)
            }
        } else {
            new.push_str(line)
        }
        new.push('\n')
    }

    *raw = new.into_bytes()
}
