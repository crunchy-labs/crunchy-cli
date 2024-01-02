use anyhow::Result;
use crunchyroll_rs::{
    Concert, Episode, MediaCollection, Movie, MovieListing, MusicVideo, Season, Series,
};

pub trait Filter {
    type T: Send + Sized;
    type Output: Send + Sized;

    async fn visit_series(&mut self, series: Series) -> Result<Vec<Season>>;
    async fn visit_season(&mut self, season: Season) -> Result<Vec<Episode>>;
    async fn visit_episode(&mut self, episode: Episode) -> Result<Option<Self::T>>;
    async fn visit_movie_listing(&mut self, movie_listing: MovieListing) -> Result<Vec<Movie>>;
    async fn visit_movie(&mut self, movie: Movie) -> Result<Option<Self::T>>;
    async fn visit_music_video(&mut self, music_video: MusicVideo) -> Result<Option<Self::T>>;
    async fn visit_concert(&mut self, concert: Concert) -> Result<Option<Self::T>>;

    async fn visit(mut self, media_collection: MediaCollection) -> Result<Self::Output>
    where
        Self: Send + Sized,
    {
        let mut items = vec![media_collection];
        let mut result = vec![];

        while !items.is_empty() {
            let mut new_items: Vec<MediaCollection> = vec![];

            for i in items {
                match i {
                    MediaCollection::Series(series) => new_items.extend(
                        self.visit_series(series)
                            .await?
                            .into_iter()
                            .map(|s| s.into())
                            .collect::<Vec<MediaCollection>>(),
                    ),
                    MediaCollection::Season(season) => new_items.extend(
                        self.visit_season(season)
                            .await?
                            .into_iter()
                            .map(|s| s.into())
                            .collect::<Vec<MediaCollection>>(),
                    ),
                    MediaCollection::Episode(episode) => {
                        if let Some(t) = self.visit_episode(episode).await? {
                            result.push(t)
                        }
                    }
                    MediaCollection::MovieListing(movie_listing) => new_items.extend(
                        self.visit_movie_listing(movie_listing)
                            .await?
                            .into_iter()
                            .map(|m| m.into())
                            .collect::<Vec<MediaCollection>>(),
                    ),
                    MediaCollection::Movie(movie) => {
                        if let Some(t) = self.visit_movie(movie).await? {
                            result.push(t)
                        }
                    }
                    MediaCollection::MusicVideo(music_video) => {
                        if let Some(t) = self.visit_music_video(music_video).await? {
                            result.push(t)
                        }
                    }
                    MediaCollection::Concert(concert) => {
                        if let Some(t) = self.visit_concert(concert).await? {
                            result.push(t)
                        }
                    }
                }
            }

            items = new_items
        }

        self.finish(result).await
    }

    async fn finish(self, input: Vec<Self::T>) -> Result<Self::Output>;
}

/// Remove all duplicates from a [`Vec`].
pub fn real_dedup_vec<T: Clone + Eq>(input: &mut Vec<T>) {
    let mut dedup = vec![];
    for item in input.clone() {
        if !dedup.contains(&item) {
            dedup.push(item);
        }
    }
    *input = dedup
}
