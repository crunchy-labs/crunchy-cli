use crate::download::filter::DownloadFilter;
use crate::utils::context::Context;
use crate::utils::download::{DownloadBuilder, DownloadFormat};
use crate::utils::ffmpeg::FFmpegPreset;
use crate::utils::filter::Filter;
use crate::utils::format::{Format, SingleFormat};
use crate::utils::log::progress;
use crate::utils::os::{free_file, has_ffmpeg, is_special_file};
use crate::utils::parse::parse_url;
use crate::utils::video::variant_data_from_stream;
use crate::Execute;
use anyhow::bail;
use anyhow::Result;
use crunchyroll_rs::media::Resolution;
use crunchyroll_rs::Locale;
use log::{debug, warn};
use std::path::Path;

#[derive(Clone, Debug, clap::Parser)]
#[clap(about = "Download a video")]
#[command(arg_required_else_help(true))]
pub struct Download {
    #[arg(help = format!("Audio language. Can only be used if the provided url(s) point to a series. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Audio language. Can only be used if the provided url(s) point to a series. \
    Available languages are:\n{}", Locale::all().into_iter().map(|l| format!("{:<6} → {}", l.to_string(), l.to_human_readable())).collect::<Vec<String>>().join("\n  ")))]
    #[arg(short, long, default_value_t = crate::utils::locale::system_locale())]
    pub(crate) audio: Locale,
    #[arg(help = format!("Subtitle language. Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Subtitle language. If set, the subtitle will be burned into the video and cannot be disabled. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(short, long)]
    pub(crate) subtitle: Option<Locale>,

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
    #[arg(short, long, default_value = "{title}.mp4")]
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

    #[arg(help = format!("Presets for video converting. Can be used multiple times. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long_help = format!("Presets for video converting. Can be used multiple times. \
    Generally used to minify the file size with keeping (nearly) the same quality. \
    It is recommended to only use this if you download videos with high resolutions since low resolution videos tend to result in a larger file with any of the provided presets. \
    Available presets: \n  {}", FFmpegPreset::available_matches_human_readable().join("\n  ")))]
    #[arg(long)]
    #[arg(value_parser = FFmpegPreset::parse)]
    pub(crate) ffmpeg_preset: Option<FFmpegPreset>,

    #[arg(help = "Skip files which are already existing")]
    #[arg(long, default_value_t = false)]
    pub(crate) skip_existing: bool,

    #[arg(help = "Url(s) to Crunchyroll episodes or series")]
    pub(crate) urls: Vec<String>,
}

#[async_trait::async_trait(?Send)]
impl Execute for Download {
    fn pre_check(&mut self) -> Result<()> {
        if !has_ffmpeg() {
            bail!("FFmpeg is needed to run this command")
        } else if Path::new(&self.output)
            .extension()
            .unwrap_or_default()
            .is_empty()
            && !is_special_file(&self.output)
            && self.output != "-"
        {
            bail!("No file extension found. Please specify a file extension (via `-o`) for the output file")
        }

        if self.subtitle.is_some() {
            if let Some(ext) = Path::new(&self.output).extension() {
                if ext.to_string_lossy() != "mp4" {
                    warn!("Detected a non mp4 output container. Adding subtitles may take a while")
                }
            }
        }

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
            let single_format_collection = DownloadFilter::new(url_filter, self.clone())
                .visit(media_collection)
                .await?;

            if single_format_collection.is_empty() {
                progress_handler.stop(format!("Skipping url {} (no matching videos found)", i + 1));
                continue;
            }
            progress_handler.stop(format!("Loaded series information for url {}", i + 1));

            single_format_collection.full_visual_output();

            let download_builder = DownloadBuilder::new()
                .default_subtitle(self.subtitle.clone())
                .output_format(if is_special_file(&self.output) || self.output == "-" {
                    Some("mpegts".to_string())
                } else {
                    None
                });

            for mut single_formats in single_format_collection.into_iter() {
                // the vec contains always only one item
                let single_format = single_formats.remove(0);

                let (download_format, format) = get_format(&self, &single_format).await?;

                let mut downloader = download_builder.clone().build();
                downloader.add_format(download_format);

                let formatted_path = format.format_path((&self.output).into(), true);
                let (path, changed) = free_file(formatted_path.clone());

                if changed && self.skip_existing {
                    debug!(
                        "Skipping already existing file '{}'",
                        formatted_path.to_string_lossy()
                    );
                    continue;
                }

                format.visual_output(&path);

                downloader.download(&ctx, &path).await?
            }
        }

        Ok(())
    }
}

async fn get_format(
    download: &Download,
    single_format: &SingleFormat,
) -> Result<(DownloadFormat, Format)> {
    let stream = single_format.stream().await?;
    let Some((video, audio)) = variant_data_from_stream(&stream, &download.resolution).await? else {
        if single_format.is_episode() {
            bail!(
                "Resolution ({}) is not available for episode {} ({}) of {} season {}",
                download.resolution,
                single_format.episode_number,
                single_format.title,
                single_format.series_name,
                single_format.season_number,
            )
        } else {
            bail!(
                "Resolution ({}) is not available for {} ({})",
                download.resolution,
                single_format.source_type(),
                single_format.title
            )
        }
    };

    let subtitle = if let Some(subtitle_locale) = &download.subtitle {
        stream.subtitles.get(subtitle_locale).map(|s| s.clone())
    } else {
        None
    };

    let download_format = DownloadFormat {
        video: (video.clone(), single_format.audio.clone()),
        audios: vec![(audio, single_format.audio.clone())],
        subtitles: subtitle.clone().map_or(vec![], |s| {
            vec![(
                s,
                single_format.audio == Locale::ja_JP || stream.subtitles.len() > 1,
            )]
        }),
    };
    let format = Format::from_single_formats(vec![(
        single_format.clone(),
        video,
        subtitle.map_or(vec![], |s| {
            vec![(
                s,
                single_format.audio == Locale::ja_JP || stream.subtitles.len() > 1,
            )]
        }),
    )]);

    Ok((download_format, format))
}
