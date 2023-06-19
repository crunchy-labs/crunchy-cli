use crate::archive::filter::ArchiveFilter;
use crate::utils::context::Context;
use crate::utils::download::{DownloadBuilder, DownloadFormat, MergeBehavior};
use crate::utils::ffmpeg::FFmpegPreset;
use crate::utils::filter::Filter;
use crate::utils::format::{Format, SingleFormat};
use crate::utils::locale::all_locale_in_locales;
use crate::utils::log::progress;
use crate::utils::os::{free_file, has_ffmpeg, is_special_file};
use crate::utils::parse::parse_url;
use crate::utils::video::variant_data_from_stream;
use crate::Execute;
use anyhow::bail;
use anyhow::Result;
use chrono::Duration;
use crunchyroll_rs::media::{Resolution, Subtitle};
use crunchyroll_rs::Locale;
use log::{debug, warn};
use std::collections::HashMap;
use std::path::PathBuf;

#[derive(Clone, Debug, clap::Parser)]
#[clap(about = "Archive a video")]
#[command(arg_required_else_help(true))]
pub struct Archive {
    #[arg(help = format!("Audio languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Audio languages. Can be used multiple times. \
    Available languages are:\n  {}", Locale::all().into_iter().map(|l| format!("{:<6} → {}", l.to_string(), l.to_human_readable())).collect::<Vec<String>>().join("\n  ")))]
    #[arg(short, long, default_values_t = vec![Locale::ja_JP, crate::utils::locale::system_locale()])]
    pub(crate) audio: Vec<Locale>,
    #[arg(help = "Deprecated. Use '-a' / '--audio' instead")]
    #[arg(short, long)]
    locale: Vec<Locale>,
    #[arg(help = format!("Subtitle languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Subtitle languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(short, long, default_values_t = Locale::all())]
    pub(crate) subtitle: Vec<Locale>,

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
      {relative_episode_number} → Number of the episode relative to its season\n  \
      {series_id}               → ID of the series\n  \
      {season_id}               → ID of the season\n  \
      {episode_id}              → ID of the episode")]
    #[arg(short, long, default_value = "{title}.mkv")]
    pub(crate) output: String,

    #[arg(help = "Video resolution")]
    #[arg(long_help = "The video resolution.\
    Can either be specified via the pixels (e.g. 1920x1080), the abbreviation for pixels (e.g. 1080p) or 'common-use' words (e.g. best). \
    Specifying the exact pixels is not recommended, use one of the other options instead. \
    Crunchyroll let you choose the quality with pixel abbreviation on their clients, so you might be already familiar with the available options. \
    The available common-use words are 'best' (choose the best resolution available) and 'worst' (worst resolution available)")]
    #[arg(short, long, default_value = "best")]
    #[arg(value_parser = crate::utils::clap::clap_parse_resolution)]
    pub(crate) resolution: Resolution,

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
    pub(crate) merge: MergeBehavior,

    #[arg(help = format!("Presets for video converting. Can be used multiple times. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long_help = format!("Presets for video converting. Can be used multiple times. \
    Generally used to minify the file size with keeping (nearly) the same quality. \
    It is recommended to only use this if you archive videos with high resolutions since low resolution videos tend to result in a larger file with any of the provided presets. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long)]
    #[arg(value_parser = FFmpegPreset::parse)]
    pub(crate) ffmpeg_preset: Option<FFmpegPreset>,

    #[arg(
        help = "Set which subtitle language should be set as default / auto shown when starting a video"
    )]
    #[arg(long)]
    pub(crate) default_subtitle: Option<Locale>,

    #[arg(help = "Skip files which are already existing")]
    #[arg(long, default_value_t = false)]
    pub(crate) skip_existing: bool,

    #[arg(help = "Crunchyroll series url(s)")]
    pub(crate) urls: Vec<String>,
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
            && !is_special_file(&self.output)
            && self.output != "-"
        {
            bail!("File extension is not '.mkv'. Currently only matroska / '.mkv' files are supported")
        }

        if !self.locale.is_empty() {
            warn!("The '-l' / '--locale' flag is deprecated, use '-a' / '--audio' instead");
            for locale in &self.locale {
                if !self.audio.contains(locale) {
                    self.audio.push(locale.clone())
                }
            }
        }

        self.audio = all_locale_in_locales(self.audio.clone());
        self.subtitle = all_locale_in_locales(self.subtitle.clone());

        Ok(())
    }

    async fn execute(self, ctx: Context) -> Result<()> {
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
            let single_format_collection = ArchiveFilter::new(url_filter, self.clone())
                .visit(media_collection)
                .await?;

            if single_format_collection.is_empty() {
                progress_handler.stop(format!("Skipping url {} (no matching videos found)", i + 1));
                continue;
            }
            progress_handler.stop(format!("Loaded series information for url {}", i + 1));

            single_format_collection.full_visual_output();

            let download_builder = DownloadBuilder::new()
                .default_subtitle(self.default_subtitle.clone())
                .ffmpeg_preset(self.ffmpeg_preset.clone().unwrap_or_default())
                .output_format(Some("matroska".to_string()))
                .audio_sort(Some(self.audio.clone()))
                .subtitle_sort(Some(self.subtitle.clone()));

            for single_formats in single_format_collection.into_iter() {
                let (download_formats, mut format) = get_format(&self, &single_formats).await?;

                let mut downloader = download_builder.clone().build();
                for download_format in download_formats {
                    downloader.add_format(download_format)
                }

                let formatted_path = format.format_path((&self.output).into(), true);
                let (path, changed) = free_file(formatted_path.clone());

                if changed && self.skip_existing {
                    debug!(
                        "Skipping already existing file '{}'",
                        formatted_path.to_string_lossy()
                    );
                    continue;
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

                downloader.download(&ctx, &path).await?
            }
        }

        Ok(())
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
        let Some((video, audio)) = variant_data_from_stream(&stream, &archive.resolution).await? else {
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
            .filter_map(|s| {
                stream
                    .subtitles
                    .get(s)
                    .cloned()
                    // the subtitle is probably not cc if the audio is japanese or more than one
                    // subtitle exists for this stream
                    .map(|l| {
                        (
                            l,
                            single_format.audio == Locale::ja_JP || stream.subtitles.len() > 1,
                        )
                    })
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
                })
            }
        }
        MergeBehavior::Audio => download_formats.push(DownloadFormat {
            video: (
                (*format_pairs.first().unwrap()).1.clone(),
                (*format_pairs.first().unwrap()).0.audio.clone(),
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
        }),
        MergeBehavior::Auto => {
            let mut d_formats: HashMap<Duration, DownloadFormat> = HashMap::new();

            for (single_format, video, audio, subtitles) in format_pairs {
                if let Some(d_format) = d_formats.get_mut(&single_format.duration) {
                    d_format.audios.push((audio, single_format.audio.clone()));
                    d_format.subtitles.extend(subtitles)
                } else {
                    d_formats.insert(
                        single_format.duration,
                        DownloadFormat {
                            video: (video, single_format.audio.clone()),
                            audios: vec![(audio, single_format.audio.clone())],
                            subtitles,
                        },
                    );
                }
            }

            for d_format in d_formats.into_values() {
                download_formats.push(d_format)
            }
        }
    }

    Ok((
        download_formats,
        Format::from_single_formats(single_format_to_format_pairs),
    ))
}
