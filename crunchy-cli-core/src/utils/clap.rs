use crate::utils::parse::parse_resolution;
use crunchyroll_rs::media::Resolution;
use regex::Regex;
use reqwest::Proxy;

pub fn clap_parse_resolution(s: &str) -> Result<Resolution, String> {
    parse_resolution(s.to_string()).map_err(|e| e.to_string())
}

pub fn clap_parse_proxies(s: &str) -> Result<(Option<Proxy>, Option<Proxy>), String> {
    let double_proxy_regex =
        Regex::new(r"^(?P<first>(https?|socks5h?)://.+):(?P<second>(https?|socks5h?)://.+)$")
            .unwrap();

    if let Some(capture) = double_proxy_regex.captures(s) {
        // checks if the input is formatted like 'https://example.com:socks5://examples.com' and
        // splits the string into 2 separate proxies at the middle colon

        let first = capture.name("first").unwrap().as_str();
        let second = capture.name("second").unwrap().as_str();
        Ok((
            Some(Proxy::all(first).map_err(|e| format!("first proxy: {e}"))?),
            Some(Proxy::all(second).map_err(|e| format!("second proxy: {e}"))?),
        ))
    } else if s.starts_with(':') {
        // checks if the input is formatted like ':https://example.com' and returns a proxy on the
        // second tuple position
        Ok((
            None,
            Some(Proxy::all(s.trim_start_matches(':')).map_err(|e| e.to_string())?),
        ))
    } else if s.ends_with(':') {
        // checks if the input is formatted like 'https://example.com:' and returns a proxy on the
        // first tuple position
        Ok((
            Some(Proxy::all(s.trim_end_matches(':')).map_err(|e| e.to_string())?),
            None,
        ))
    } else {
        // returns the same proxy for both tuple positions
        let proxy = Proxy::all(s).map_err(|e| e.to_string())?;
        Ok((Some(proxy.clone()), Some(proxy)))
    }
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
