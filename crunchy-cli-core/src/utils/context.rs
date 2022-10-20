use crunchyroll_rs::Crunchyroll;

pub struct Context {
    pub crunchy: Crunchyroll,
    pub client: isahc::HttpClient,
}
