use crate::utils::parse::parse_resolution;
use crunchyroll_rs::media::Resolution;
use reqwest::Proxy;

pub fn clap_parse_resolution(s: &str) -> Result<Resolution, String> {
    parse_resolution(s.to_string()).map_err(|e| e.to_string())
}

pub fn clap_parse_proxy(s: &str) -> Result<Proxy, String> {
    Proxy::all(s).map_err(|e| e.to_string())
}
