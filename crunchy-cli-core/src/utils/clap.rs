use crate::utils::parse::parse_resolution;
use crunchyroll_rs::media::Resolution;
use reqwest::Proxy;

pub fn clap_parse_resolution(s: &str) -> Result<Resolution, String> {
    parse_resolution(s.to_string()).map_err(|e| e.to_string())
}

pub fn clap_parse_proxy(s: &str) -> Result<Proxy, String> {
    Proxy::all(s).map_err(|e| e.to_string())
}

pub fn clap_parse_speed_limit(s: &str) -> Result<u32, String> {
    let quota = s.to_lowercase();

    let bytes = if let Ok(b) = quota.parse() {
        b
    } else if let Ok(b) = quota.trim_end_matches('b').parse::<u32>() {
        b
    } else if let Ok(kb) = quota.trim_end_matches("kb").parse::<u32>() {
        kb * 1024
    } else if let Ok(mb) = quota.trim_end_matches("mb").parse::<u32>() {
        mb * 1024 * 1024
    } else {
        return Err("Invalid speed limit".to_string());
    };
    Ok(bytes)
}
