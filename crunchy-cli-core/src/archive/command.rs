use crate::archive::filter::ArchiveFilter;
use crate::utils::context::Context;
use crate::utils::download::MergeBehavior;
use crate::utils::ffmpeg::FFmpegPreset;
use crate::utils::filter::Filter;
use crate::utils::format::formats_visual_output;
use crate::utils::locale::all_locale_in_locales;
use crate::utils::log::progress;
use crate::utils::os::{free_file, has_ffmpeg, is_special_file};
use crate::utils::parse::parse_url;
use crate::Execute;
use anyhow::bail;
use anyhow::Result;
use crunchyroll_rs::media::Resolution;
use crunchyroll_rs::Locale;
use log::debug;
use std::path::PathBuf;

#[derive(Clone, Debug, clap::Parser)]
#[clap(about = "Archive a video")]
#[command(arg_required_else_help(true))]
#[command()]
pub struct Archive {
    #[arg(help = format!("Audio languages. Can be used multiple times. \
    Available languages are: {}", Locale::all().into_iter().map(|l| l.to_string()).collect::<Vec<String>>().join(", ")))]
    #[arg(long_help = format!("Audio languages. Can be used multiple times. \
    Available languages are:\n{}", Locale::all().into_iter().map(|l| format!("{:<6} → {}", l.to_string(), l.to_human_readable())).collect::<Vec<String>>().join("\n  ")))]
    #[arg(short, long, default_values_t = vec![Locale::ja_JP, crate::utils::locale::system_locale()])]
    pub(crate) locale: Vec<Locale>,
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
      {relative_episode_number} → Number of the episode relative to its season\
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
            && !is_special_file(PathBuf::from(&self.output))
            && self.output != "-"
        {
            bail!("File extension is not '.mkv'. Currently only matroska / '.mkv' files are supported")
        }

        self.locale = all_locale_in_locales(self.locale.clone());
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
            let archive_formats = ArchiveFilter::new(url_filter, self.clone())
                .visit(media_collection)
                .await?;

            if archive_formats.is_empty() {
                progress_handler.stop(format!("Skipping url {} (no matching videos found)", i + 1));
                continue;
            }
            progress_handler.stop(format!("Loaded series information for url {}", i + 1));

            formats_visual_output(archive_formats.iter().map(|(_, f)| f).collect());

            for (downloader, mut format) in archive_formats {
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
                    self.locale
                        .iter()
                        .position(|l| l == a)
                        .cmp(&self.locale.iter().position(|l| l == b))
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
