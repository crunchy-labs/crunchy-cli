use crate::cli::log::tab_info;
use crate::cli::utils::{download_segments, find_resolution};
use crate::utils::context::Context;
use crate::utils::format::{format_string, Format};
use crate::utils::log::progress;
use crate::utils::os::{free_file, has_ffmpeg, tempfile};
use crate::utils::parse::{parse_url, UrlFilter};
use crate::utils::sort::{sort_formats_after_seasons, sort_seasons_after_number};
use crate::Execute;
use anyhow::{bail, Result};
use crunchyroll_rs::media::{Resolution, StreamSubtitle};
use crunchyroll_rs::{Locale, Media, MediaCollection, Series};
use log::{debug, error, info};
use regex::Regex;
use std::collections::BTreeMap;
use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Stdio};
use tempfile::TempPath;

#[derive(Clone, Debug)]
pub enum MergeBehavior {
    Auto,
    Audio,
    Video,
}

fn parse_merge_behavior(s: &str) -> Result<MergeBehavior, String> {
    Ok(match s.to_lowercase().as_str() {
        "auto" => MergeBehavior::Auto,
        "audio" => MergeBehavior::Audio,
        "video" => MergeBehavior::Video,
        _ => return Err(format!("'{}' is not a valid merge behavior", s)),
    })
}

#[derive(Debug, clap::Parser)]
#[clap(about = "Archive a video")]
#[command(arg_required_else_help(true))]
#[command()]
pub struct Archive {
    #[arg(help = format!("Audio languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Audio languages. Can be used multiple times. \
    Available languages are:\n{}", Locale::all().into_iter().map(|l| format!("{:<6} → {}", l.to_string(), l.to_human_readable())).collect::<Vec<String>>().join("\n  ")))]
    #[arg(short, long, default_values_t = vec![crate::utils::locale::system_locale(), Locale::ja_JP])]
    locale: Vec<Locale>,
    #[arg(help = format!("Subtitle languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Subtitle languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(short, long, default_values_t = Locale::all())]
    subtitle: Vec<Locale>,

    #[arg(help = "Name of the output file")]
    #[arg(long_help = "Name of the output file.\
    If you use one of the following pattern they will get replaced:\n  \
      {title}          → Title of the video\n  \
      {series_name}    → Name of the series\n  \
      {season_name}    → Name of the season\n  \
      {audio}          → Audio language of the video\n  \
      {resolution}     → Resolution of the video\n  \
      {season_number}  → Number of the season\n  \
      {episode_number} → Number of the episode\n  \
      {series_id}      → ID of the series\n  \
      {season_id}      → ID of the season\n  \
      {episode_id}     → ID of the episode")]
    #[arg(short, long, default_value = "{title}.mkv")]
    output: String,

    #[arg(help = "Video resolution")]
    #[arg(long_help = "The video resolution.\
    Can either be specified via the pixels (e.g. 1920x1080), the abbreviation for pixels (e.g. 1080p) or 'common-use' words (e.g. best). \
    Specifying the exact pixels is not recommended, use one of the other options instead. \
    Crunchyroll let you choose the quality with pixel abbreviation on their clients, so you might be already familiar with the available options. \
    The available common-use words are 'best' (choose the best resolution available) and 'worst' (worst resolution available)")]
    #[arg(short, long, default_value = "best")]
    #[arg(value_parser = crate::utils::clap::clap_parse_resolution)]
    resolution: Resolution,

    #[arg(
        help = "Sets the behavior of the stream merging. Valid behaviors are 'auto', 'audio' and 'video'"
    )]
    #[arg(
        long_help = "Because of local restrictions (or other reasons) some episodes with different languages does not have the same length (e.g. when some scenes were cut out). \
    With this flag you can set the behavior when handling multiple language.
    Valid options are 'audio' (stores one video and all other languages as audio only), 'video' (stores the video + audio for every language) and 'auto' (detects if videos differ in length: if so, behave like 'video' else like 'audio')"
    )]
    #[arg(short, long, default_value = "auto")]
    #[arg(value_parser = parse_merge_behavior)]
    merge: MergeBehavior,

    #[arg(
        help = "Set which subtitle language should be set as default / auto shown when starting a video"
    )]
    #[arg(long)]
    default_subtitle: Option<Locale>,
    #[arg(help = "Disable subtitle optimizations")]
    #[arg(
        long_help = "By default, Crunchyroll delivers subtitles in a format which may cause issues in some video players. \
    These issues are fixed internally by setting a flag which is not part of the official specification of the subtitle format. \
    If you do not want this fixes or they cause more trouble than they solve (for you), it can be disabled with this flag"
    )]
    #[arg(long)]
    no_subtitle_optimizations: bool,

    #[arg(help = "Crunchyroll series url(s)")]
    urls: Vec<String>,
}

#[async_trait::async_trait(?Send)]
impl Execute for Archive {
    fn pre_check(&self) -> Result<()> {
        if !has_ffmpeg() {
            bail!("FFmpeg is needed to run this command")
        } else if PathBuf::from(&self.output).extension().unwrap_or_default().to_string_lossy() != "mkv" {
            bail!("File extension is not '.mkv'. Currently only matroska / '.mkv' files are supported")
        }

        Ok(())
    }

    async fn execute(self, ctx: Context) -> Result<()> {
        let mut parsed_urls = vec![];

        for (i, url) in self.urls.iter().enumerate() {
            let _progress_handler = progress!("Parsing url {}", i + 1);
            match parse_url(&ctx.crunchy, url.clone(), true).await {
                Ok((media_collection, url_filter)) => {
                    parsed_urls.push((media_collection, url_filter));
                    info!("Parsed url {}", i + 1)
                }
                Err(e) => bail!("url {} could not be parsed: {}", url, e),
            }
        }

        for (i, (media_collection, url_filter)) in parsed_urls.into_iter().enumerate() {
            let archive_formats = match media_collection {
                MediaCollection::Series(series) => {
                    let _progress_handler = progress!("Fetching series details");
                    formats_from_series(&self, series, &url_filter).await?
                }
                MediaCollection::Season(_) => bail!("Archiving a season is not supported"),
                MediaCollection::Episode(episode) => bail!("Archiving a episode is not supported. Use url filtering instead to specify the episode (https://www.crunchyroll.com/series/{}/{}[S{}E{}])", episode.metadata.series_id, episode.metadata.series_slug_title, episode.metadata.season_number, episode.metadata.episode_number),
                MediaCollection::MovieListing(_) => bail!("Archiving a movie listing is not supported"),
                MediaCollection::Movie(_) => bail!("Archiving a movie is not supported")
            };

            if archive_formats.is_empty() {
                info!("Skipping url {} (no matching episodes found)", i + 1);
                continue;
            }
            info!("Loaded series information for url {}", i + 1);

            if log::max_level() == log::Level::Debug {
                let seasons = sort_formats_after_seasons(
                    archive_formats
                        .clone()
                        .into_iter()
                        .map(|(a, _)| a.get(0).unwrap().clone())
                        .collect(),
                );
                debug!("Series has {} seasons", seasons.len());
                for (i, season) in seasons.into_iter().enumerate() {
                    info!("Season {} ({})", i + 1, season.get(0).unwrap().season_title);
                    for format in season {
                        info!(
                            "{}: {}px, {:.02} FPS (S{:02}E{:02})",
                            format.title,
                            format.stream.resolution,
                            format.stream.fps,
                            format.season_number,
                            format.number,
                        )
                    }
                }
            } else {
                for season in sort_formats_after_seasons(
                    archive_formats
                        .clone()
                        .into_iter()
                        .map(|(a, _)| a.get(0).unwrap().clone())
                        .collect(),
                ) {
                    let first = season.get(0).unwrap();
                    info!(
                        "{} Season {} ({})",
                        first.series_name, first.season_number, first.season_title
                    );

                    for (i, format) in season.into_iter().enumerate() {
                        tab_info!(
                            "{}. {} » {}px, {:.2} FPS (S{:02}E{:02})",
                            i + 1,
                            format.title,
                            format.stream.resolution,
                            format.stream.fps,
                            format.season_number,
                            format.number
                        )
                    }
                }
            }

            for (formats, subtitles) in archive_formats {
                let (primary, additionally) = formats.split_first().unwrap();

                let mut path = PathBuf::from(&self.output);
                path = free_file(
                    path.with_file_name(format_string(
                        if let Some(fname) = path.file_name() {
                            fname.to_str().unwrap()
                        } else {
                            "{title}.mkv"
                        }
                        .to_string(),
                        primary,
                    )),
                );

                info!(
                    "Downloading {} to '{}'",
                    primary.title,
                    path.to_str().unwrap()
                );
                tab_info!(
                    "Episode: S{:02}E{:02}",
                    primary.season_number,
                    primary.number
                );
                tab_info!(
                    "Audio: {} (primary), {}",
                    primary.audio,
                    additionally
                        .iter()
                        .map(|a| a.audio.to_string())
                        .collect::<Vec<String>>()
                        .join(", ")
                );
                tab_info!(
                    "Subtitle: {}",
                    subtitles
                        .iter()
                        .map(|s| {
                            if let Some(default) = &self.default_subtitle {
                                if default == &s.locale {
                                    return format!("{} (primary)", default);
                                }
                            }
                            s.locale.to_string()
                        })
                        .collect::<Vec<String>>()
                        .join(", ")
                );
                tab_info!("Resolution: {}", primary.stream.resolution);
                tab_info!("FPS: {:.2}", primary.stream.fps);

                let mut video_paths = vec![];
                let mut audio_paths = vec![];
                let mut subtitle_paths = vec![];

                video_paths.push((download_video(&ctx, primary, false).await?, primary));
                for additional in additionally {
                    let only_audio = match self.merge {
                        MergeBehavior::Auto => additionally
                            .iter()
                            .all(|a| a.stream.bandwidth == primary.stream.bandwidth),
                        MergeBehavior::Audio => true,
                        MergeBehavior::Video => false,
                    };
                    let path = download_video(&ctx, additional, only_audio).await?;
                    if only_audio {
                        audio_paths.push((path, additional))
                    } else {
                        video_paths.push((path, additional))
                    }
                }

                for subtitle in subtitles {
                    subtitle_paths
                        .push((download_subtitle(&self, subtitle.clone()).await?, subtitle))
                }

                generate_mkv(&self, path, video_paths, audio_paths, subtitle_paths)?
            }
        }

        Ok(())
    }
}

async fn formats_from_series(
    archive: &Archive,
    series: Media<Series>,
    url_filter: &UrlFilter,
) -> Result<Vec<(Vec<Format>, Vec<StreamSubtitle>)>> {
    let mut seasons = series.seasons().await?;

    // filter any season out which does not contain the specified audio languages
    for season in sort_seasons_after_number(seasons.clone()) {
        // get all locales which are specified but not present in the current iterated season and
        // print an error saying this
        let not_present_audio = archive
            .locale
            .clone()
            .into_iter()
            .filter(|l| !season.iter().any(|s| &s.metadata.audio_locale == l))
            .collect::<Vec<Locale>>();
        for not_present in not_present_audio {
            error!(
                "Season {} of series {} is not available with {} audio",
                season.first().unwrap().metadata.season_number,
                series.title,
                not_present
            )
        }

        // remove all seasons with the wrong audio for the current iterated season number
        seasons.retain(|s| {
            s.metadata.season_number != season.first().unwrap().metadata.season_number
                || archive.locale.contains(&s.metadata.audio_locale)
        })
    }

    #[allow(clippy::type_complexity)]
    let mut result: BTreeMap<u32, BTreeMap<u32, (Vec<Format>, Vec<StreamSubtitle>)>> =
        BTreeMap::new();
    for season in series.seasons().await? {
        if !url_filter.is_season_valid(season.metadata.season_number)
            || !archive.locale.contains(&season.metadata.audio_locale)
        {
            continue;
        }

        for episode in season.episodes().await? {
            if !url_filter.is_episode_valid(
                episode.metadata.episode_number,
                episode.metadata.season_number,
            ) {
                continue;
            }

            let streams = episode.streams().await?;
            let streaming_data = streams.hls_streaming_data(None).await?;
            let Some(stream) = find_resolution(streaming_data, &archive.resolution) else {
                bail!(
                    "Resolution ({}x{}) is not available for episode {} ({}) of season {} ({}) of {}",
                    archive.resolution.width,
                    archive.resolution.height,
                    episode.metadata.episode_number,
                    episode.title,
                    episode.metadata.season_number,
                    episode.metadata.season_title,
                    episode.metadata.series_title
                )
            };

            let (ref mut formats, _) = result
                .entry(season.metadata.season_number)
                .or_insert_with(BTreeMap::new)
                .entry(episode.metadata.episode_number)
                .or_insert_with(|| {
                    let subtitles: Vec<StreamSubtitle> = archive
                        .subtitle
                        .iter()
                        .filter_map(|l| streams.subtitles.get(l).cloned())
                        .collect();
                    (vec![], subtitles)
                });
            formats.push(Format::new_from_episode(episode, stream));
        }
    }

    Ok(result.into_values().flat_map(|v| v.into_values()).collect())
}

async fn download_video(ctx: &Context, format: &Format, only_audio: bool) -> Result<TempPath> {
    let tempfile = if only_audio {
        tempfile(".aac")?
    } else {
        tempfile(".ts")?
    };
    let (_, path) = tempfile.into_parts();

    let ffmpeg = Command::new("ffmpeg")
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .arg("-y")
        .args(["-f", "mpegts", "-i", "pipe:"])
        .args(if only_audio { vec!["-vn"] } else { vec![] })
        .arg(path.to_str().unwrap())
        .spawn()?;

    download_segments(
        ctx,
        &mut ffmpeg.stdin.unwrap(),
        Some(format!("Download {}", format.audio)),
        format.stream.clone(),
    )
    .await?;

    Ok(path)
}

async fn download_subtitle(archive: &Archive, subtitle: StreamSubtitle) -> Result<TempPath> {
    let tempfile = tempfile(".ass")?;
    let (mut file, path) = tempfile.into_parts();

    let mut buf = vec![];
    subtitle.write_to(&mut buf).await?;
    if !archive.no_subtitle_optimizations {
        buf = fix_subtitle(buf)
    }

    file.write_all(buf.as_slice())?;

    Ok(path)
}

/// Add `ScaledBorderAndShadows: yes` to subtitles; without it they look very messy on some video
/// players. See [crunchy-labs/crunchy-cli#66](https://github.com/crunchy-labs/crunchy-cli/issues/66)
/// for more information.
fn fix_subtitle(raw: Vec<u8>) -> Vec<u8> {
    let mut script_info = false;
    let mut new = String::new();

    for line in String::from_utf8_lossy(raw.as_slice()).split('\n') {
        if line.trim().starts_with('[') && script_info {
            new.push_str("ScaledBorderAndShadows: yes\n");
            script_info = false
        } else if line.trim() == "[Script Info]" {
            script_info = true
        }
        new.push_str(line);
        new.push('\n')
    }

    new.into_bytes()
}

fn generate_mkv(
    archive: &Archive,
    target: PathBuf,
    video_paths: Vec<(TempPath, &Format)>,
    audio_paths: Vec<(TempPath, &Format)>,
    subtitle_paths: Vec<(TempPath, StreamSubtitle)>,
) -> Result<()> {
    let mut input = vec![];
    let mut maps = vec![];
    let mut metadata = vec![];

    let mut video_length = (0, 0, 0, 0);

    for (i, (video_path, format)) in video_paths.iter().enumerate() {
        input.extend(["-i".to_string(), video_path.to_string_lossy().to_string()]);
        maps.extend(["-map".to_string(), i.to_string()]);
        metadata.extend([
            format!("-metadata:s:v:{}", i),
            format!("language={}", format.audio),
        ]);
        metadata.extend([
            format!("-metadata:s:v:{}", i),
            format!("title={}", format.audio.to_human_readable()),
        ]);
        metadata.extend([
            format!("-metadata:s:a:{}", i),
            format!("language={}", format.audio),
        ]);
        metadata.extend([
            format!("-metadata:s:a:{}", i),
            format!("title={}", format.audio.to_human_readable()),
        ]);

        let vid_len = get_video_length(video_path.to_path_buf())?;
        if vid_len > video_length {
            video_length = vid_len
        }
    }
    for (i, (audio_path, format)) in audio_paths.iter().enumerate() {
        input.extend(["-i".to_string(), audio_path.to_string_lossy().to_string()]);
        maps.extend(["-map".to_string(), (i + video_paths.len()).to_string()]);
        metadata.extend([
            format!("-metadata:s:a:{}", i + video_paths.len()),
            format!("language={}", format.audio),
        ]);
        metadata.extend([
            format!("-metadata:s:a:{}", i + video_paths.len()),
            format!("title={}", format.audio.to_human_readable()),
        ]);
    }
    for (i, (subtitle_path, subtitle)) in subtitle_paths.iter().enumerate() {
        input.extend([
            "-i".to_string(),
            subtitle_path.to_string_lossy().to_string(),
        ]);
        maps.extend([
            "-map".to_string(),
            (i + video_paths.len() + audio_paths.len()).to_string(),
        ]);
        metadata.extend([
            format!("-metadata:s:s:{}", i),
            format!("language={}", subtitle.locale),
        ]);
        metadata.extend([
            format!("-metadata:s:s:{}", i),
            format!("title={}", subtitle.locale.to_human_readable()),
        ]);
    }

    let mut command_args = vec!["-y".to_string()];
    command_args.extend(input);
    command_args.extend(maps);
    command_args.extend(metadata);

    // set default subtitle
    if let Some(default_subtitle) = &archive.default_subtitle {
        // if `--default_subtitle <locale>` is given set the default subtitle to the given locale
        if let Some(position) = subtitle_paths
            .into_iter()
            .position(|s| &s.1.locale == default_subtitle)
        {
            command_args.push(format!("-disposition:s:{}", position))
        } else {
            command_args.extend(["-disposition:s:0".to_string(), "0".to_string()])
        }
    } else {
        command_args.extend(["-disposition:s:0".to_string(), "0".to_string()])
    }

    command_args.extend([
        "-c".to_string(),
        "copy".to_string(),
        "-f".to_string(),
        "matroska".to_string(),
        target.to_string_lossy().to_string(),
    ]);

    debug!("ffmpeg {}", command_args.join(" "));

    let ffmpeg = Command::new("ffmpeg")
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .args(command_args)
        .output()?;
    if !ffmpeg.status.success() {
        bail!("{}", String::from_utf8_lossy(ffmpeg.stderr.as_slice()))
    }

    Ok(())
}

/// Get the length of a video. This is required because sometimes subtitles have an unnecessary entry
/// long after the actual video ends with artificially extends the video length on some video players.
/// To prevent this, the video length must be hard set with ffmpeg. See
/// [crunchy-labs/crunchy-cli#32](https://github.com/crunchy-labs/crunchy-cli/issues/32) for more
/// information.
fn get_video_length(path: PathBuf) -> Result<(u32, u32, u32, u32)> {
    let video_length = Regex::new(r"Duration:\s?(\d+):(\d+):(\d+).(\d+),")?;

    let ffmpeg = Command::new("ffmpeg")
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .arg("-y")
        .args(["-i", path.to_str().unwrap()])
        .output()?;
    let ffmpeg_output = String::from_utf8(ffmpeg.stderr)?;
    let caps = video_length.captures(ffmpeg_output.as_str()).unwrap();

    Ok((
        caps[1].parse()?,
        caps[2].parse()?,
        caps[3].parse()?,
        caps[4].parse()?,
    ))
}
