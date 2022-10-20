use crate::utils::format::Format;
use crunchyroll_rs::{Media, Season};
use std::collections::BTreeMap;

/// Sort seasons after their season number. Crunchyroll may have multiple seasons for one season
/// number. They generally store different language in individual seasons with the same season number.
/// E.g. series X has one official season but crunchy has translations for it in 3 different languages
/// so there exist 3 different "seasons" on Crunchyroll which are actual the same season but with
/// different audio.
pub fn sort_seasons_after_number(seasons: Vec<Media<Season>>) -> Vec<Vec<Media<Season>>> {
    let mut as_map = BTreeMap::new();

    for season in seasons {
        as_map
            .entry(season.metadata.season_number)
            .or_insert_with(Vec::new);
        as_map
            .get_mut(&season.metadata.season_number)
            .unwrap()
            .push(season)
    }

    as_map.into_values().collect()
}

/// Sort formats after their seasons and episodes (inside it) ascending. Make sure to have only
/// episodes from one series and in one language as argument since the function does not handle those
/// differences which could then lead to a semi messed up result.
pub fn sort_formats_after_seasons(formats: Vec<Format>) -> Vec<Vec<Format>> {
    let mut as_map = BTreeMap::new();

    for format in formats {
        as_map.entry(format.season_number).or_insert_with(Vec::new);
        as_map.get_mut(&format.season_number).unwrap().push(format);
    }

    let mut sorted = as_map
        .into_iter()
        .map(|(_, mut values)| {
            values.sort_by(|a, b| a.number.cmp(&b.number));
            values
        })
        .collect::<Vec<Vec<Format>>>();
    sorted.sort_by(|a, b| a[0].series_id.cmp(&b[0].series_id));

    sorted
}
