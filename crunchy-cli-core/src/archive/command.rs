use crate::archive::filter::ArchiveFilter;
use crate::utils::context::Context;
use crate::utils::download::{
    DownloadBuilder, DownloadFormat, DownloadFormatMetadata, MergeBehavior,
};
use crate::utils::ffmpeg::FFmpegPreset;
use crate::utils::filter::Filter;
use crate::utils::format::{Format, SingleFormat};
use crate::utils::locale::{all_locale_in_locales, resolve_locales, LanguageTagging};
use crate::utils::log::progress;
use crate::utils::os::{free_file, has_ffmpeg, is_special_file};
use crate::utils::parse::parse_url;
use crate::utils::video::stream_data_from_stream;
use crate::Execute;
use anyhow::bail;
use anyhow::Result;
use chrono::Duration;
use crunchyroll_rs::media::{Resolution, Subtitle};
use crunchyroll_rs::Locale;
use log::{debug, warn};
use regex::Regex;
use std::fmt::{Display, Formatter};
use std::iter::zip;
use std::ops::Sub;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

#[derive(Clone, Debug, clap::Parser)]
#[clap(about = "Archive a video")]
#[command(arg_required_else_help(true))]
pub struct Archive {
    #[arg(help = format!("Audio languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Audio languages. Can be used multiple times. \
    Available languages are:\n  {}\nIETF tagged language codes for the shown available locales can be used too", Locale::all().into_iter().map(|l| format!("{:<6} → {}", l.to_string(), l.to_human_readable())).collect::<Vec<String>>().join("\n  ")))]
    #[arg(short, long, default_values_t = vec![Locale::ja_JP, crate::utils::locale::system_locale()])]
    pub(crate) audio: Vec<Locale>,
    #[arg(skip)]
    output_audio_locales: Vec<String>,
    #[arg(help = format!("Subtitle languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Subtitle languages. Can be used multiple times. \
    Available languages are: {}\nIETF tagged language codes for the shown available locales can be used too", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(short, long, default_values_t = Locale::all())]
    pub(crate) subtitle: Vec<Locale>,
    #[arg(skip)]
    output_subtitle_locales: Vec<String>,

    #[arg(help = "Name of the output file")]
    #[arg(long_help = "Name of the output file. \
    If you use one of the following pattern they will get replaced:\n  \
      {title}                    → Title of the video\n  \
      {series_name}              → Name of the series\n  \
      {season_name}              → Name of the season\n  \
      {audio}                    → Audio language of the video\n  \
      {width}                    → Width of the video\n  \
      {height}                   → Height of the video\n  \
      {season_number}            → Number of the season\n  \
      {episode_number}           → Number of the episode\n  \
      {relative_episode_number}  → Number of the episode relative to its season\n  \
      {sequence_number}          → Like '{episode_number}' but without possible non-number characters\n  \
      {relative_sequence_number} → Like '{relative_episode_number}' but with support for episode 0's and .5's\n  \
      {release_year}             → Release year of the video\n  \
      {release_month}            → Release month of the video\n  \
      {release_day}              → Release day of the video\n  \
      {series_id}                → ID of the series\n  \
      {season_id}                → ID of the season\n  \
      {episode_id}               → ID of the episode")]
    #[arg(short, long, default_value = "{title}.mkv")]
    pub(crate) output: String,
    #[arg(help = "Name of the output file if the episode is a special")]
    #[arg(long_help = "Name of the output file if the episode is a special. \
    If not set, the '-o'/'--output' flag will be used as name template")]
    #[arg(long)]
    pub(crate) output_specials: Option<String>,

    #[arg(help = "Sanitize the output file for use with all operating systems. \
    This option only affects template options and not static characters.")]
    #[arg(long, default_value_t = false)]
    pub(crate) universal_output: bool,

    #[arg(help = "Video resolution")]
    #[arg(long_help = "The video resolution. \
    Can either be specified via the pixels (e.g. 1920x1080), the abbreviation for pixels (e.g. 1080p) or 'common-use' words (e.g. best). \
    Specifying the exact pixels is not recommended, use one of the other options instead. \
    Crunchyroll let you choose the quality with pixel abbreviation on their clients, so you might be already familiar with the available options. \
    The available common-use words are 'best' (choose the best resolution available) and 'worst' (worst resolution available)")]
    #[arg(short, long, default_value = "best")]
    #[arg(value_parser = crate::utils::clap::clap_parse_resolution)]
    pub(crate) resolution: Resolution,

    #[arg(
        help = "Sets the behavior of the stream merging. Valid behaviors are 'auto', 'sync', 'audio' and 'video'"
    )]
    #[arg(
        long_help = "Because of local restrictions (or other reasons) some episodes with different languages does not have the same length (e.g. when some scenes were cut out). \
    With this flag you can set the behavior when handling multiple language.
    Valid options are 'audio' (stores one video and all other languages as audio only), 'video' (stores the video + audio for every language), 'auto' (detects if videos differ in length: if so, behave like 'video' else like 'audio') and 'sync' (detects if videos differ in length: if so, tries to find the offset of matching audio parts and removes it from the beginning, otherwise it behaves like 'audio')"
    )]
    #[arg(short, long, default_value = "auto")]
    #[arg(value_parser = MergeBehavior::parse)]
    pub(crate) merge: MergeBehavior,
    #[arg(
        help = "If the merge behavior is 'auto' or 'sync', consider videos to be of equal lengths if the difference in length is smaller than the specified milliseconds"
    )]
    #[arg(long, default_value_t = 200)]
    pub(crate) merge_time_tolerance: u32,
    #[arg(
        help = "If the merge behavior is 'sync', specify the difference by which two fingerprints are considered equal, higher values can help when the algorithm fails"
    )]
    #[arg(long, default_value_t = 6)]
    pub(crate) merge_sync_tolerance: u32,
    #[arg(
        help = "If the merge behavior is 'sync', specify the amount of offset determination runs from which the final offset is calculated, higher values will increase the time required but lead to more precise offsets"
    )]
    #[arg(long, default_value_t = 4)]
    pub(crate) merge_sync_precision: u32,

    #[arg(
        help = "Specified which language tagging the audio and subtitle tracks and language specific format options should have. \
        Valid options are: 'default' (how Crunchyroll uses it internally), 'ietf' (according to the IETF standard)"
    )]
    #[arg(
        long_help = "Specified which language tagging the audio and subtitle tracks and language specific format options should have. \
        Valid options are: 'default' (how Crunchyroll uses it internally), 'ietf' (according to the IETF standard; you might run in issues as there are multiple locales which resolve to the same IETF language code, e.g. 'es-LA' and 'es-ES' are both resolving to 'es')"
    )]
    #[arg(long)]
    #[arg(value_parser = LanguageTagging::parse)]
    pub(crate) language_tagging: Option<LanguageTagging>,

    #[arg(help = format!("Presets for converting the video to a specific coding format. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long_help = format!("Presets for converting the video to a specific coding format. \
    If you need more specific ffmpeg customizations you can pass ffmpeg output arguments instead of a preset as value. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long)]
    #[arg(value_parser = FFmpegPreset::parse)]
    pub(crate) ffmpeg_preset: Option<FFmpegPreset>,
    #[arg(
        help = "The number of threads used by ffmpeg to generate the output file. Does not work with every codec/preset"
    )]
    #[arg(
        long_help = "The number of threads used by ffmpeg to generate the output file. \
    Does not work with every codec/preset and is skipped entirely when specifying custom ffmpeg output arguments instead of a preset for `--ffmpeg-preset`. \
    By default, ffmpeg chooses the thread count which works best for the output codec"
    )]
    #[arg(long)]
    pub(crate) ffmpeg_threads: Option<usize>,

    #[arg(
        help = "Set which subtitle language should be set as default / auto shown when starting a video"
    )]
    #[arg(long)]
    pub(crate) default_subtitle: Option<Locale>,
    #[arg(help = "Include fonts in the downloaded file")]
    #[arg(long)]
    pub(crate) include_fonts: bool,
    #[arg(
        help = "Includes chapters (e.g. intro, credits, ...). Only works if `--merge` is set to 'audio'"
    )]
    #[arg(
        long_help = "Includes chapters (e.g. intro, credits, ...). . Only works if `--merge` is set to 'audio'. \
    Because chapters are essentially only special timeframes in episodes like the intro, most of the video timeline isn't covered by a chapter.
    These \"gaps\" are filled with an 'Episode' chapter because many video players are ignore those gaps and just assume that a chapter ends when the next chapter start is reached, even if a specific end-time is set.
    Also chapters aren't always available, so in this case, just a big 'Episode' chapter from start to end will be created"
    )]
    #[arg(long, default_value_t = false)]
    pub(crate) include_chapters: bool,

    #[arg(help = "Omit closed caption subtitles in the downloaded file")]
    #[arg(long, default_value_t = false)]
    pub(crate) no_closed_caption: bool,

    #[arg(help = "Skip files which are already existing by their name")]
    #[arg(long, default_value_t = false)]
    pub(crate) skip_existing: bool,
    #[arg(
        help = "Only works in combination with `--skip-existing`. Sets the method how already existing files should be skipped. Valid methods are 'audio' and 'subtitle'"
    )]
    #[arg(long_help = "Only works in combination with `--skip-existing`. \
    By default, already existing files are determined by their name and the download of the corresponding episode is skipped. \
    With this flag you can modify this behavior. \
    Valid options are 'audio' and 'subtitle' (if the file already exists but the audio/subtitle are less from what should be downloaded, the episode gets downloaded and the file overwritten).")]
    #[arg(long, default_values_t = SkipExistingMethod::default())]
    #[arg(value_parser = SkipExistingMethod::parse)]
    pub(crate) skip_existing_method: Vec<SkipExistingMethod>,
    #[arg(help = "Skip special episodes")]
    #[arg(long, default_value_t = false)]
    pub(crate) skip_specials: bool,

    #[arg(help = "Skip any interactive input")]
    #[arg(short, long, default_value_t = false)]
    pub(crate) yes: bool,

    #[arg(help = "The number of threads used to download")]
    #[arg(short, long, default_value_t = num_cpus::get())]
    pub(crate) threads: usize,

    #[arg(help = "Crunchyroll series url(s)")]
    #[arg(required = true)]
    pub(crate) urls: Vec<String>,
}

impl Execute for Archive {
    fn pre_check(&mut self) -> Result<()> {
        if !has_ffmpeg() {
            bail!("FFmpeg is needed to run this command")
        } else if PathBuf::from(&self.output)
            .extension()
            .unwrap_or_default()
            .to_string_lossy()
            != "mkv"
            && !is_special_file(&self.output)
            && self.output != "-"
        {
            bail!("File extension is not '.mkv'. Currently only matroska / '.mkv' files are supported")
        } else if let Some(special_output) = &self.output_specials {
            if PathBuf::from(special_output)
                .extension()
                .unwrap_or_default()
                .to_string_lossy()
                != "mkv"
                && !is_special_file(special_output)
                && special_output != "-"
            {
                bail!("File extension for special episodes is not '.mkv'. Currently only matroska / '.mkv' files are supported")
            }
        }

        if self.include_chapters
            && !matches!(self.merge, MergeBehavior::Sync)
            && !matches!(self.merge, MergeBehavior::Audio)
        {
            bail!("`--include-chapters` can only be used if `--merge` is set to 'audio' or 'sync'")
        }

        self.audio = all_locale_in_locales(self.audio.clone());
        self.subtitle = all_locale_in_locales(self.subtitle.clone());

        if let Some(language_tagging) = &self.language_tagging {
            self.audio = resolve_locales(&self.audio);
            self.subtitle = resolve_locales(&self.subtitle);
            self.output_audio_locales = language_tagging.convert_locales(&self.audio);
            self.output_subtitle_locales = language_tagging.convert_locales(&self.subtitle);
        } else {
            self.output_audio_locales = self
                .audio
                .clone()
                .into_iter()
                .map(|l| l.to_string())
                .collect();
            self.output_subtitle_locales = self
                .subtitle
                .clone()
                .into_iter()
                .map(|l| l.to_string())
                .collect();
        }

        Ok(())
    }

    async fn execute(self, ctx: Context) -> Result<()> {
        if !ctx.crunchy.premium().await {
            warn!("You may not be able to download all requested videos when logging in anonymously or using a non-premium account")
        }

        let mut parsed_urls = vec![];

        for (i, url) in self.urls.clone().into_iter().enumerate() {
            let progress_handler = progress!("Parsing url {}", i + 1);
            match parse_url(&ctx.crunchy, url.clone(), true).await {
                Ok((media_collection, url_filter)) => {
                    progress_handler.stop(format!("Parsed url {}", i + 1));
                    parsed_urls.push((media_collection, url_filter))
                }
                Err(e) => bail!("url {} could not be parsed: {}", url, e),
            };
        }

        for (i, (media_collection, url_filter)) in parsed_urls.into_iter().enumerate() {
            let progress_handler = progress!("Fetching series details");
            let single_format_collection = ArchiveFilter::new(
                url_filter,
                self.clone(),
                !self.yes,
                self.skip_specials,
                ctx.crunchy.premium().await,
            )
            .visit(media_collection)
            .await?;

            if single_format_collection.is_empty() {
                progress_handler.stop(format!("Skipping url {} (no matching videos found)", i + 1));
                continue;
            }
            progress_handler.stop(format!("Loaded series information for url {}", i + 1));

            single_format_collection.full_visual_output();

            let download_builder =
                DownloadBuilder::new(ctx.client.clone(), ctx.rate_limiter.clone())
                    .default_subtitle(self.default_subtitle.clone())
                    .download_fonts(self.include_fonts)
                    .ffmpeg_preset(self.ffmpeg_preset.clone().unwrap_or_default())
                    .ffmpeg_threads(self.ffmpeg_threads)
                    .output_format(Some("matroska".to_string()))
                    .audio_sort(Some(self.audio.clone()))
                    .subtitle_sort(Some(self.subtitle.clone()))
                    .no_closed_caption(self.no_closed_caption)
                    .merge_sync_tolerance(match self.merge {
                        MergeBehavior::Sync => Some(self.merge_sync_tolerance),
                        _ => None,
                    })
                    .merge_sync_precision(match self.merge {
                        MergeBehavior::Sync => Some(self.merge_sync_precision),
                        _ => None,
                    })
                    .threads(self.threads)
                    .audio_locale_output_map(
                        zip(self.audio.clone(), self.output_audio_locales.clone()).collect(),
                    )
                    .subtitle_locale_output_map(
                        zip(self.subtitle.clone(), self.output_subtitle_locales.clone()).collect(),
                    );

            for single_formats in single_format_collection.into_iter() {
                let (download_formats, mut format) = get_format(&self, &single_formats).await?;

                let mut downloader = download_builder.clone().build();
                for download_format in download_formats {
                    downloader.add_format(download_format)
                }

                let formatted_path = if format.is_special() {
                    format.format_path(
                        self.output_specials
                            .as_ref()
                            .map_or((&self.output).into(), |so| so.into()),
                        self.universal_output,
                        self.language_tagging.as_ref(),
                    )
                } else {
                    format.format_path(
                        (&self.output).into(),
                        self.universal_output,
                        self.language_tagging.as_ref(),
                    )
                };
                let (mut path, changed) = free_file(formatted_path.clone());

                if changed && self.skip_existing {
                    let mut skip = true;

                    if !self.skip_existing_method.is_empty() {
                        if let Some((audio_locales, subtitle_locales)) =
                            get_video_streams(&formatted_path)?
                        {
                            let method_audio = self
                                .skip_existing_method
                                .contains(&SkipExistingMethod::Audio);
                            let method_subtitle = self
                                .skip_existing_method
                                .contains(&SkipExistingMethod::Subtitle);

                            let audio_differ = if method_audio {
                                format
                                    .locales
                                    .iter()
                                    .any(|(a, _)| !audio_locales.contains(a))
                            } else {
                                false
                            };
                            let subtitle_differ = if method_subtitle {
                                format
                                    .locales
                                    .clone()
                                    .into_iter()
                                    .flat_map(|(a, mut s)| {
                                        // remove the closed caption if the flag is given to omit
                                        // closed captions
                                        if self.no_closed_caption && a != Locale::ja_JP {
                                            s.retain(|l| l != &a)
                                        }
                                        s
                                    })
                                    .any(|l| !subtitle_locales.contains(&l))
                            } else {
                                false
                            };

                            if (method_audio && audio_differ)
                                || (method_subtitle && subtitle_differ)
                            {
                                skip = false;
                                path.clone_from(&formatted_path)
                            }
                        }
                    }

                    if skip {
                        debug!(
                            "Skipping already existing file '{}'",
                            formatted_path.to_string_lossy()
                        );
                        continue;
                    }
                }

                format.locales.sort_by(|(a, _), (b, _)| {
                    self.audio
                        .iter()
                        .position(|l| l == a)
                        .cmp(&self.audio.iter().position(|l| l == b))
                });
                for (_, subtitles) in format.locales.iter_mut() {
                    subtitles.sort_by(|a, b| {
                        self.subtitle
                            .iter()
                            .position(|l| l == a)
                            .cmp(&self.subtitle.iter().position(|l| l == b))
                    })
                }

                format.visual_output(&path);

                downloader.download(&path).await?
            }
        }

        Ok(())
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum SkipExistingMethod {
    Audio,
    Subtitle,
}

impl Display for SkipExistingMethod {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        let value = match self {
            SkipExistingMethod::Audio => "audio",
            SkipExistingMethod::Subtitle => "subtitle",
        };
        write!(f, "{}", value)
    }
}

impl SkipExistingMethod {
    fn parse(s: &str) -> Result<Self, String> {
        match s.to_lowercase().as_str() {
            "audio" => Ok(Self::Audio),
            "subtitle" => Ok(Self::Subtitle),
            _ => Err(format!("invalid skip existing method '{}'", s)),
        }
    }

    fn default<'a>() -> &'a [Self] {
        &[]
    }
}

async fn get_format(
    archive: &Archive,
    single_formats: &Vec<SingleFormat>,
) -> Result<(Vec<DownloadFormat>, Format)> {
    let mut format_pairs = vec![];
    let mut single_format_to_format_pairs = vec![];

    for single_format in single_formats {
        let stream = single_format.stream().await?;
        let Some((video, audio, _)) =
            stream_data_from_stream(&stream, &archive.resolution, None).await?
        else {
            if single_format.is_episode() {
                bail!(
                    "Resolution ({}) is not available for episode {} ({}) of {} season {}",
                    archive.resolution,
                    single_format.episode_number,
                    single_format.title,
                    single_format.series_name,
                    single_format.season_number,
                )
            } else {
                bail!(
                    "Resolution ({}) is not available for {} ({})",
                    archive.resolution,
                    single_format.source_type(),
                    single_format.title
                )
            }
        };

        let subtitles: Vec<(Subtitle, bool)> = archive
            .subtitle
            .iter()
            .flat_map(|s| {
                let subtitles = stream
                    .subtitles
                    .get(s)
                    .cloned()
                    // the subtitle is probably cc if the audio is not japanese or only one
                    // subtitle exists for this stream
                    .map(|l| {
                        (
                            l,
                            single_format.audio != Locale::ja_JP && stream.subtitles.len() == 1,
                        )
                    });
                let cc = stream.captions.get(s).cloned().map(|l| (l, true));

                subtitles
                    .into_iter()
                    .chain(cc.into_iter())
                    .collect::<Vec<(Subtitle, bool)>>()
            })
            .collect();

        format_pairs.push((single_format, video.clone(), audio, subtitles.clone()));
        single_format_to_format_pairs.push((single_format.clone(), video, subtitles))
    }

    let mut download_formats = vec![];

    match archive.merge {
        MergeBehavior::Video => {
            for (single_format, video, audio, subtitles) in format_pairs {
                download_formats.push(DownloadFormat {
                    video: (video, single_format.audio.clone()),
                    audios: vec![(audio, single_format.audio.clone())],
                    subtitles,
                    metadata: DownloadFormatMetadata { skip_events: None },
                })
            }
        }
        MergeBehavior::Audio => download_formats.push(DownloadFormat {
            video: (
                format_pairs.first().unwrap().1.clone(),
                format_pairs.first().unwrap().0.audio.clone(),
            ),
            audios: format_pairs
                .iter()
                .map(|(single_format, _, audio, _)| (audio.clone(), single_format.audio.clone()))
                .collect(),
            // mix all subtitles together and then reduce them via a map so that only one subtitle
            // per language exists
            subtitles: format_pairs
                .iter()
                .flat_map(|(_, _, _, subtitles)| subtitles.clone())
                .collect(),
            metadata: DownloadFormatMetadata {
                skip_events: if archive.include_chapters {
                    format_pairs.first().unwrap().0.skip_events().await?
                } else {
                    None
                },
            },
        }),
        MergeBehavior::Auto | MergeBehavior::Sync => {
            let mut d_formats: Vec<(Duration, DownloadFormat)> = vec![];

            for (single_format, video, audio, subtitles) in format_pairs {
                let closest_format = d_formats.iter_mut().min_by(|(x, _), (y, _)| {
                    x.sub(single_format.duration)
                        .abs()
                        .cmp(&y.sub(single_format.duration).abs())
                });

                match closest_format {
                    Some(closest_format)
                        if closest_format
                            .0
                            .sub(single_format.duration)
                            .abs()
                            .num_milliseconds()
                            < archive.merge_time_tolerance.into() =>
                    {
                        // If less than `audio_error` apart, use same audio.
                        closest_format
                            .1
                            .audios
                            .push((audio, single_format.audio.clone()));
                        closest_format.1.subtitles.extend(subtitles);
                    }
                    _ => {
                        d_formats.push((
                            single_format.duration,
                            DownloadFormat {
                                video: (video, single_format.audio.clone()),
                                audios: vec![(audio, single_format.audio.clone())],
                                subtitles,
                                metadata: DownloadFormatMetadata {
                                    skip_events: if archive.include_chapters {
                                        single_format.skip_events().await?
                                    } else {
                                        None
                                    },
                                },
                            },
                        ));
                    }
                };
            }

            for (_, d_format) in d_formats.into_iter() {
                download_formats.push(d_format);
            }
        }
    }

    Ok((
        download_formats,
        Format::from_single_formats(single_format_to_format_pairs),
    ))
}

fn get_video_streams(path: &Path) -> Result<Option<(Vec<Locale>, Vec<Locale>)>> {
    let video_streams =
        Regex::new(r"(?m)Stream\s#\d+:\d+\((?P<language>.+)\):\s(?P<type>(Audio|Subtitle))")
            .unwrap();

    let ffmpeg = Command::new("ffmpeg")
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .arg("-hide_banner")
        .args(["-i", &path.to_string_lossy()])
        .output()?;
    let ffmpeg_output = String::from_utf8(ffmpeg.stderr)?;

    let mut audio = vec![];
    let mut subtitle = vec![];
    for cap in video_streams.captures_iter(&ffmpeg_output) {
        let locale = cap.name("language").unwrap().as_str();
        let type_ = cap.name("type").unwrap().as_str();

        match type_ {
            "Audio" => audio.push(Locale::from(locale.to_string())),
            "Subtitle" => subtitle.push(Locale::from(locale.to_string())),
            _ => unreachable!(),
        }
    }

    if audio.is_empty() && subtitle.is_empty() {
        Ok(None)
    } else {
        Ok(Some((audio, subtitle)))
    }
}
