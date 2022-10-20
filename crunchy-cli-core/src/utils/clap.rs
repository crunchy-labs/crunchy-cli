use crate::utils::parse::parse_resolution;
use crunchyroll_rs::media::Resolution;

pub fn clap_parse_resolution(s: &str) -> Result<Resolution, String> {
    parse_resolution(s.to_string()).map_err(|e| e.to_string())
}
