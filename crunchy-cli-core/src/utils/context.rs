use crate::utils::rate_limit::RateLimiterService;
use crunchyroll_rs::Crunchyroll;
use reqwest::Client;

pub struct Context {
    pub crunchy: Crunchyroll,
    pub client: Client,
    pub rate_limiter: Option<RateLimiterService>,
}
