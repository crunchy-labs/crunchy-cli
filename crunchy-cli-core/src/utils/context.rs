use crate::utils::config::Config;
use crunchyroll_rs::Crunchyroll;

pub struct Context {
    pub crunchy: Crunchyroll,
    pub config: Config,
}
