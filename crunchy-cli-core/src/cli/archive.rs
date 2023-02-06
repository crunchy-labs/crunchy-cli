use crate::cli::log::tab_info;
use crate::cli::utils::{
    all_locale_in_locales, download_segments, find_multiple_seasons_with_same_number,
    find_resolution, interactive_season_choosing, FFmpegPreset,
};
use crate::utils::context::Context;
use crate::utils::format::Format;
use crate::utils::log::progress;
use crate::utils::os::{free_file, has_ffmpeg, is_special_file, tempfile};
use crate::utils::parse::{parse_url, UrlFilter};
use crate::utils::sort::{sort_formats_after_seasons, sort_seasons_after_number};
use crate::utils::subtitle::{download_subtitle, Subtitle};
use crate::utils::video::get_video_length;
use crate::Execute;
use anyhow::{bail, Result};
use crunchyroll_rs::media::Resolution;
use crunchyroll_rs::{Locale, Media, MediaCollection, Series};
use log::{debug, error, info};
use std::collections::BTreeMap;
use std::path::PathBuf;
use std::process::{Command, Stdio};
use tempfile::TempPath;

#[derive(Clone, Debug)]
pub enum MergeBehavior {
    Auto,
    Audio,
    Video,
}

impl MergeBehavior {
    fn parse(s: &str) -> Result<MergeBehavior, String> {
        Ok(match s.to_lowercase().as_str() {
            "auto" => MergeBehavior::Auto,
            "audio" => MergeBehavior::Audio,
            "video" => MergeBehavior::Video,
            _ => return Err(format!("'{}' is not a valid merge behavior", s)),
        })
    }
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
      {title}                   → Title of the video\n  \
      {series_name}             → Name of the series\n  \
      {season_name}             → Name of the season\n  \
      {audio}                   → Audio language of the video\n  \
      {resolution}              → Resolution of the video\n  \
      {season_number}           → Number of the season\n  \
      {episode_number}          → Number of the episode\n  \
      {relative_episode_number} → Number of the episode relative to its season\
      {series_id}               → ID of the series\n  \
      {season_id}               → ID of the season\n  \
      {episode_id}              → ID of the episode")]
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
    #[arg(value_parser = MergeBehavior::parse)]
    merge: MergeBehavior,

    #[arg(help = format!("Presets for video converting. Can be used multiple times. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long_help = format!("Presets for video converting. Can be used multiple times. \
    Generally used to minify the file size with keeping (nearly) the same quality. \
    It is recommended to only use this if you archive videos with high resolutions since low resolution videos tend to result in a larger file with any of the provided presets. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long)]
    #[arg(value_parser = FFmpegPreset::parse)]
    ffmpeg_preset: Option<FFmpegPreset>,

    #[arg(
        help = "Set which subtitle language should be set as default / auto shown when starting a video"
    )]
    #[arg(long)]
    default_subtitle: Option<Locale>,

    #[arg(help = "Skip files which are already existing")]
    #[arg(long, default_value_t = false)]
    skip_existing: bool,

    #[arg(help = "Ignore interactive input")]
    #[arg(short, long, default_value_t = false)]
    yes: bool,

    #[arg(help = "Crunchyroll series url(s)")]
    urls: Vec<String>,
}

#[async_trait::async_trait(?Send)]
impl Execute for Archive {
    fn pre_check(&mut self) -> Result<()> {
        if !has_ffmpeg() {
            bail!("FFmpeg is needed to run this command")
        } else if PathBuf::from(&self.output)
            .extension()
            .unwrap_or_default()
            .to_string_lossy()
            != "mkv"
            && !is_special_file(PathBuf::from(&self.output))
        {
            bail!("File extension is not '.mkv'. Currently only matroska / '.mkv' files are supported")
        }

        self.locale = all_locale_in_locales(self.locale.clone());
        self.subtitle = all_locale_in_locales(self.subtitle.clone());

        Ok(())
    }

    async fn execute(self, ctx: Context) -> Result<()> {
        let mut parsed_urls = vec![];

        for (i, url) in self.urls.iter().enumerate() {
            let progress_handler = progress!("Parsing url {}", i + 1);
            match parse_url(&ctx.crunchy, url.clone(), true).await {
                Ok((media_collection, url_filter)) => {
                    parsed_urls.push((media_collection, url_filter));
                    progress_handler.stop(format!("Parsed url {}", i + 1))
                }
                Err(e) => bail!("url {} could not be parsed: {}", url, e),
            }
        }

        for (i, (media_collection, url_filter)) in parsed_urls.into_iter().enumerate() {
            let progress_handler = progress!("Fetching series details");
            let archive_formats = match media_collection {
                MediaCollection::Series(series) => {
                    formats_from_series(&self, series, &url_filter).await?
                }
                MediaCollection::Season(_) => bail!("Archiving a season is not supported"),
                MediaCollection::Episode(episode) => bail!("Archiving a episode is not supported. Use url filtering instead to specify the episode (https://www.crunchyroll.com/series/{}/{}[S{}E{}])", episode.metadata.series_id, episode.metadata.series_slug_title, episode.metadata.season_number, episode.metadata.episode_number),
                MediaCollection::MovieListing(_) => bail!("Archiving a movie listing is not supported"),
                MediaCollection::Movie(_) => bail!("Archiving a movie is not supported")
            };

            if archive_formats.is_empty() {
                progress_handler.stop(format!(
                    "Skipping url {} (no matching episodes found)",
                    i + 1
                ));
                continue;
            }
            progress_handler.stop(format!("Loaded series information for url {}", i + 1));

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
                            format.episode_number,
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
                            format.episode_number
                        )
                    }
                }
            }

            for (formats, mut subtitles) in archive_formats {
                let (primary, additionally) = formats.split_first().unwrap();

                let formatted_path = primary.format_path((&self.output).into(), true);
                let (path, changed) = free_file(formatted_path.clone());

                if changed && self.skip_existing {
                    debug!(
                        "Skipping already existing file '{}'",
                        formatted_path.to_string_lossy()
                    );
                    continue;
                }

                info!(
                    "Downloading {} to '{}'",
                    primary.title,
                    if is_special_file(&path) {
                        path.to_str().unwrap()
                    } else {
                        path.file_name().unwrap().to_str().unwrap()
                    }
                );
                tab_info!(
                    "Episode: S{:02}E{:02}",
                    primary.season_number,
                    primary.episode_number
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
                        .filter(|s| s.primary) // Don't print subtitles of non-primary streams. They might get removed depending on the merge behavior.
                        .map(|s| {
                            if let Some(default) = &self.default_subtitle {
                                if default == &s.stream_subtitle.locale {
                                    return format!("{} (primary)", default);
                                }
                            }
                            s.stream_subtitle.locale.to_string()
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
                    let identical_video = additionally
                        .iter()
                        .all(|a| a.stream.bandwidth == primary.stream.bandwidth);
                    let only_audio = match self.merge {
                        MergeBehavior::Auto => identical_video,
                        MergeBehavior::Audio => true,
                        MergeBehavior::Video => false,
                    };
                    let path = download_video(&ctx, additional, only_audio).await?;
                    if only_audio {
                        audio_paths.push((path, additional))
                    } else {
                        video_paths.push((path, additional))
                    }

                    // Remove subtitles of forcibly deleted video
                    if matches!(self.merge, MergeBehavior::Audio) && !identical_video {
                        subtitles.retain(|s| s.episode_id != additional.episode_id);
                    }
                }

                let (primary_video, _) = video_paths.get(0).unwrap();
                let primary_video_length = get_video_length(primary_video.to_path_buf()).unwrap();
                for subtitle in subtitles {
                    subtitle_paths.push((
                        download_subtitle(subtitle.stream_subtitle.clone(), primary_video_length)
                            .await?,
                        subtitle,
                    ))
                }

                let progess_handler = progress!("Generating mkv");
                generate_mkv(&self, path, video_paths, audio_paths, subtitle_paths)?;
                progess_handler.stop("Mkv generated")
            }
        }

        Ok(())
    }
}

async fn formats_from_series(
    archive: &Archive,
    series: Media<Series>,
    url_filter: &UrlFilter,
) -> Result<Vec<(Vec<Format>, Vec<Subtitle>)>> {
    let mut seasons = series.seasons().await?;

    // filter any season out which does not contain the specified audio languages
    for season in sort_seasons_after_number(seasons.clone()) {
        // get all locales which are specified but not present in the current iterated season and
        // print an error saying this
        let not_present_audio = archive
            .locale
            .clone()
            .into_iter()
            .filter(|l| !season.iter().any(|s| s.metadata.audio_locales.contains(l)))
            .collect::<Vec<Locale>>();
        if !not_present_audio.is_empty() {
            error!(
                "Season {} of series {} is not available with {} audio",
                season.first().unwrap().metadata.season_number,
                series.title,
                not_present_audio
                    .into_iter()
                    .map(|l| l.to_string())
                    .collect::<Vec<String>>()
                    .join(", ")
            )
        }

        // remove all seasons with the wrong audio for the current iterated season number
        seasons.retain(|s| {
            s.metadata.season_number != season.first().unwrap().metadata.season_number
                || archive
                    .locale
                    .iter()
                    .any(|l| s.metadata.audio_locales.contains(l))
        });
        // remove seasons which match the url filter. this is mostly done to not trigger the
        // interactive season choosing when dupilcated seasons are excluded by the filter
        seasons.retain(|s| url_filter.is_season_valid(s.metadata.season_number))
    }

    if !archive.yes && !find_multiple_seasons_with_same_number(&seasons).is_empty() {
        info!(target: "progress_end", "Fetched seasons");
        seasons = interactive_season_choosing(seasons);
        info!(target: "progress", "Fetching series details")
    }

    #[allow(clippy::type_complexity)]
    let mut result: BTreeMap<u32, BTreeMap<String, (Vec<Format>, Vec<Subtitle>)>> = BTreeMap::new();
    let mut primary_season = true;
    for season in seasons {
        let episodes = season.episodes().await?;

        for episode in episodes.iter() {
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

            let (ref mut formats, subtitles) = result
                .entry(season.metadata.season_number)
                .or_insert_with(BTreeMap::new)
                .entry(episode.metadata.episode.clone())
                .or_insert_with(|| (vec![], vec![]));
            subtitles.extend(archive.subtitle.iter().filter_map(|l| {
                let stream_subtitle = streams.subtitles.get(l).cloned()?;
                let subtitle = Subtitle {
                    stream_subtitle,
                    audio_locale: episode.metadata.audio_locale.clone(),
                    episode_id: episode.id.clone(),
                    forced: !episode.metadata.is_subbed,
                    primary: primary_season,
                };
                Some(subtitle)
            }));
            formats.push(Format::new_from_episode(episode, &episodes, stream, vec![]));
        }

        primary_season = false;
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
        .args(["-f", "mpegts"])
        .args(["-i", "pipe:"])
        .args(["-c", "copy"])
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

fn generate_mkv(
    archive: &Archive,
    target: PathBuf,
    video_paths: Vec<(TempPath, &Format)>,
    audio_paths: Vec<(TempPath, &Format)>,
    subtitle_paths: Vec<(TempPath, Subtitle)>,
) -> Result<()> {
    let mut input = vec![];
    let mut maps = vec![];
    let mut metadata = vec![];
    let mut dispositions = vec![vec![]; subtitle_paths.len()];

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
            format!("language={}", subtitle.stream_subtitle.locale),
        ]);
        metadata.extend([
            format!("-metadata:s:s:{}", i),
            format!(
                "title={}",
                subtitle.stream_subtitle.locale.to_human_readable()
                    + if !subtitle.primary {
                        format!(" [Video: {}]", subtitle.audio_locale.to_human_readable())
                    } else {
                        "".to_string()
                    }
                    .as_str()
            ),
        ]);

        // mark forced subtitles
        if subtitle.forced {
            dispositions[i].push("forced");
        }
    }

    let (input_presets, output_presets) = if let Some(preset) = archive.ffmpeg_preset.clone() {
        preset.to_input_output_args()
    } else {
        (
            vec![],
            vec![
                "-c:v".to_string(),
                "copy".to_string(),
                "-c:a".to_string(),
                "copy".to_string(),
            ],
        )
    };

    let mut command_args = vec!["-y".to_string()];
    command_args.extend(input_presets);
    command_args.extend(input);
    command_args.extend(maps);
    command_args.extend(metadata);

    // set default subtitle
    if let Some(default_subtitle) = &archive.default_subtitle {
        // if `--default_subtitle <locale>` is given set the default subtitle to the given locale
        if let Some(position) = subtitle_paths
            .iter()
            .position(|(_, subtitle)| &subtitle.stream_subtitle.locale == default_subtitle)
        {
            dispositions[position].push("default");
        }
    }

    let disposition_args: Vec<String> = dispositions
        .iter()
        .enumerate()
        .flat_map(|(i, d)| {
            vec![
                format!("-disposition:s:{}", i),
                if !d.is_empty() {
                    d.join("+")
                } else {
                    "0".to_string()
                },
            ]
        })
        .collect();
    command_args.extend(disposition_args);

    command_args.extend(output_presets);
    command_args.extend([
        "-f".to_string(),
        "matroska".to_string(),
        target.to_string_lossy().to_string(),
    ]);

    debug!("ffmpeg {}", command_args.join(" "));

    // create parent directory if it does not exist
    if let Some(parent) = target.parent() {
        if !parent.exists() {
            std::fs::create_dir_all(parent)?
        }
    }

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
