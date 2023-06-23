use crate::utils::log::progress_pause;
use crunchyroll_rs::Season;
use dialoguer::console::Term;
use dialoguer::MultiSelect;
use std::collections::BTreeMap;

pub fn get_duplicated_seasons(seasons: &Vec<Season>) -> Vec<u32> {
    let mut season_number_counter = BTreeMap::<u32, u32>::new();
    for season in seasons {
        season_number_counter
            .entry(season.season_number)
            .and_modify(|c| *c += 1)
            .or_default();
    }
    season_number_counter
        .into_iter()
        .filter_map(|(k, v)| if v > 0 { Some(k) } else { None })
        .collect()
}

pub fn check_for_duplicated_seasons(seasons: &mut Vec<Season>) {
    let mut as_map = BTreeMap::new();
    for season in seasons.iter() {
        as_map
            .entry(season.season_number)
            .or_insert(vec![])
            .push(season)
    }

    let duplicates: Vec<&Season> = as_map
        .into_values()
        .filter(|s| s.len() > 1)
        .flatten()
        .collect();
    progress_pause!();
    let _ = Term::stdout().clear_line();
    let keep = select(
        "Duplicated seasons were found. Select the one you want to download (space to select/deselect; enter to continue)",
        duplicates
            .iter()
            .map(|s| format!("Season {} ({})", s.season_number, s.title))
            .collect(),
    );
    progress_pause!();

    let mut remove_ids = vec![];
    for (i, duplicate) in duplicates.into_iter().enumerate() {
        if !keep.contains(&i) {
            remove_ids.push(duplicate.id.clone())
        }
    }

    seasons.retain(|s| !remove_ids.contains(&s.id));
}

pub fn select(prompt: &str, input: Vec<String>) -> Vec<usize> {
    if input.is_empty() {
        return vec![];
    }

    let def: Vec<bool> = (0..input.len()).map(|_| true).collect();

    let selection = MultiSelect::new()
        .with_prompt(prompt)
        .items(&input[..])
        .defaults(&def[..])
        .clear(false)
        .report(false)
        .interact_on(&Term::stdout())
        .unwrap_or_default();

    selection
}
