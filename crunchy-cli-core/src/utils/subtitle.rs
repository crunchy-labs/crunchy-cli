use crunchyroll_rs::media::StreamSubtitle;
use crunchyroll_rs::Locale;

#[derive(Clone)]
pub struct Subtitle {
    pub stream_subtitle: StreamSubtitle,
    pub audio_locale: Locale,
    pub episode_id: String,
    pub forced: bool,
    pub primary: bool,
}
