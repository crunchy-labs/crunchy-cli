use std::{
    cmp,
    collections::{HashMap, HashSet},
    ops::Not,
    path::Path,
    process::Command,
};

use chrono::TimeDelta;
use crunchyroll_rs::Locale;
use log::debug;
use tempfile::TempPath;

use anyhow::{bail, Result};

use super::fmt::format_time_delta;

pub struct SyncAudio {
    pub format_id: usize,
    pub path: TempPath,
    pub locale: Locale,
    pub video_idx: usize,
}

#[derive(Debug, Clone, Copy)]
struct TimeRange {
    start: f64,
    end: f64,
}

pub fn sync_audios(
    available_audios: &Vec<SyncAudio>,
    sync_tolerance: u32,
    sync_precision: u32,
) -> Result<Option<HashMap<usize, TimeDelta>>> {
    let mut result: HashMap<usize, TimeDelta> = HashMap::new();

    let mut sync_audios = vec![];
    let mut chromaprints = HashMap::new();
    let mut formats = HashSet::new();
    for audio in available_audios {
        if formats.contains(&audio.format_id) {
            continue;
        }
        formats.insert(audio.format_id);
        sync_audios.push((audio.format_id, &audio.path));
        chromaprints.insert(
            audio.format_id,
            generate_chromaprint(
                &audio.path,
                &TimeDelta::zero(),
                &TimeDelta::zero(),
                &TimeDelta::zero(),
            )?,
        );
    }
    sync_audios.sort_by_key(|sync_audio| chromaprints.get(&sync_audio.0).unwrap().len());

    let base_audio = sync_audios.remove(0);

    let mut start = f64::MAX;
    let mut end = f64::MIN;
    let mut initial_offsets = HashMap::new();
    for audio in &sync_audios {
        debug!(
            "Initial comparison of format {} to {}",
            audio.0, &base_audio.0
        );

        let (lhs_ranges, rhs_ranges) = compare_chromaprints(
            chromaprints.get(&base_audio.0).unwrap(),
            chromaprints.get(&audio.0).unwrap(),
            sync_tolerance,
        );
        if lhs_ranges.is_empty() || rhs_ranges.is_empty() {
            bail!(
                "Failed to sync videos, couldn't find matching audio parts between format {} and {}",
                base_audio.0 + 1,
                audio.0 + 1
            );
        }
        let lhs_range = lhs_ranges[0];
        let rhs_range = rhs_ranges[0];
        start = start.min(lhs_range.start);
        end = end.max(lhs_range.end);
        start = start.min(rhs_range.start);
        end = end.max(rhs_range.end);
        let offset = TimeDelta::milliseconds(((rhs_range.start - lhs_range.start) * 1000.0) as i64);
        initial_offsets.insert(audio.0, TimeDelta::zero().checked_sub(&offset).unwrap());
        debug!(
            "Found initial offset of {}ms ({} - {} {}s) ({} - {} {}s) for format {} to {}",
            offset.num_milliseconds(),
            lhs_range.start,
            lhs_range.end,
            lhs_range.end - lhs_range.start,
            rhs_range.start,
            rhs_range.end,
            rhs_range.end - rhs_range.start,
            audio.0,
            base_audio.0
        );
    }

    debug!(
        "Found matching audio parts at {} - {}, narrowing search",
        start, end
    );

    let start = TimeDelta::milliseconds((start * 1000.0) as i64 - 20000);
    let end = TimeDelta::milliseconds((end * 1000.0) as i64 + 20000);

    for sync_audio in &sync_audios {
        let chromaprint = generate_chromaprint(
            sync_audio.1,
            &start,
            &end,
            initial_offsets.get(&sync_audio.0).unwrap(),
        )?;
        chromaprints.insert(sync_audio.0, chromaprint);
    }

    let mut runs: HashMap<usize, i64> = HashMap::new();
    let iterator_range_limits: i64 = 2 ^ sync_precision as i64;
    for i in -iterator_range_limits..=iterator_range_limits {
        let base_offset = TimeDelta::milliseconds(
            ((0.128 / iterator_range_limits as f64 * i as f64) * 1000.0) as i64,
        );
        chromaprints.insert(
            base_audio.0,
            generate_chromaprint(base_audio.1, &start, &end, &base_offset)?,
        );
        for audio in &sync_audios {
            let initial_offset = initial_offsets.get(&audio.0).copied().unwrap();
            let offset = find_offset(
                (&base_audio.0, chromaprints.get(&base_audio.0).unwrap()),
                &base_offset,
                (&audio.0, chromaprints.get(&audio.0).unwrap()),
                &initial_offset,
                &start,
                sync_tolerance,
            );
            if offset.is_none() {
                continue;
            }
            let offset = offset.unwrap();

            result.insert(
                audio.0,
                result
                    .get(&audio.0)
                    .copied()
                    .unwrap_or_default()
                    .checked_add(&offset)
                    .unwrap(),
            );
            runs.insert(audio.0, runs.get(&audio.0).copied().unwrap_or_default() + 1);
        }
    }
    let mut result: HashMap<usize, TimeDelta> = result
        .iter()
        .map(|(format_id, offset)| {
            (
                *format_id,
                TimeDelta::milliseconds(
                    offset.num_milliseconds() / runs.get(format_id).copied().unwrap(),
                ),
            )
        })
        .collect();
    result.insert(base_audio.0, TimeDelta::milliseconds(0));

    Ok(Some(result))
}

fn find_offset(
    lhs: (&usize, &Vec<u32>),
    lhs_shift: &TimeDelta,
    rhs: (&usize, &Vec<u32>),
    rhs_shift: &TimeDelta,
    start: &TimeDelta,
    sync_tolerance: u32,
) -> Option<TimeDelta> {
    let (lhs_ranges, rhs_ranges) = compare_chromaprints(lhs.1, rhs.1, sync_tolerance);
    if lhs_ranges.is_empty() || rhs_ranges.is_empty() {
        return None;
    }
    let lhs_range = lhs_ranges[0];
    let rhs_range = rhs_ranges[0];
    let offset = rhs_range.end - lhs_range.end;
    let offset = TimeDelta::milliseconds((offset * 1000.0) as i64)
        .checked_add(lhs_shift)?
        .checked_sub(rhs_shift)?;
    debug!(
        "Found offset of {}ms ({} - {} {}s) ({} - {} {}s) for format {} to {}",
        offset.num_milliseconds(),
        lhs_range.start + start.num_milliseconds() as f64 / 1000.0,
        lhs_range.end + start.num_milliseconds() as f64 / 1000.0,
        lhs_range.end - lhs_range.start,
        rhs_range.start + start.num_milliseconds() as f64 / 1000.0,
        rhs_range.end + start.num_milliseconds() as f64 / 1000.0,
        rhs_range.end - rhs_range.start,
        rhs.0,
        lhs.0
    );
    Some(offset)
}

fn generate_chromaprint(
    input_file: &Path,
    start: &TimeDelta,
    end: &TimeDelta,
    offset: &TimeDelta,
) -> Result<Vec<u32>> {
    let mut ss_argument: &TimeDelta = &start.checked_sub(offset).unwrap();
    let mut offset_argument = &TimeDelta::zero();
    if *offset < TimeDelta::zero() {
        ss_argument = start;
        offset_argument = offset;
    };

    let mut command = Command::new("ffmpeg");
    command
        .arg("-hide_banner")
        .arg("-y")
        .args(["-ss", format_time_delta(ss_argument).as_str()]);

    if end.is_zero().not() {
        command.args(["-to", format_time_delta(end).as_str()]);
    }

    command
        .args(["-itsoffset", format_time_delta(offset_argument).as_str()])
        .args(["-i", input_file.to_string_lossy().to_string().as_str()])
        .args(["-ac", "2"])
        .args(["-f", "chromaprint"])
        .args(["-fp_format", "raw"])
        .arg("-");

    let extract_output = command.output()?;

    if !extract_output.status.success() {
        bail!(
            "{}",
            String::from_utf8_lossy(extract_output.stderr.as_slice())
        );
    }
    let raw_chromaprint = extract_output.stdout.as_slice();
    let length = raw_chromaprint.len();
    if length % 4 != 0 {
        bail!("chromaprint bytes should be a multiple of 4");
    }
    let mut chromaprint = Vec::with_capacity(length / 4);
    for i in 0..length / 4 {
        chromaprint.push(as_u32_le(
            raw_chromaprint[i * 4..i * 4 + 4].try_into().unwrap(),
        ));
    }
    Ok(chromaprint)
}

fn compare_chromaprints(
    lhs_chromaprint: &Vec<u32>,
    rhs_chromaprint: &Vec<u32>,
    sync_tolerance: u32,
) -> (Vec<TimeRange>, Vec<TimeRange>) {
    let lhs_inverse_index = create_inverse_index(lhs_chromaprint);
    let rhs_inverse_index = create_inverse_index(rhs_chromaprint);

    let mut possible_shifts = HashSet::new();
    for lhs_pair in lhs_inverse_index {
        let original_point = lhs_pair.0;
        for i in -2..=2 {
            let modified_point = (original_point as i32 + i) as u32;
            if rhs_inverse_index.contains_key(&modified_point) {
                let rhs_index = rhs_inverse_index.get(&modified_point).copied().unwrap();
                possible_shifts.insert(rhs_index as i32 - lhs_pair.1 as i32);
            }
        }
    }

    let mut all_lhs_time_ranges = vec![];
    let mut all_rhs_time_ranges = vec![];
    for shift_amount in possible_shifts {
        let time_range_pair = find_time_ranges(
            lhs_chromaprint,
            rhs_chromaprint,
            shift_amount,
            sync_tolerance,
        );
        if time_range_pair.is_none() {
            continue;
        }
        let (mut lhs_time_ranges, mut rhs_time_ranges) = time_range_pair.unwrap();
        let mut lhs_time_ranges: Vec<TimeRange> = lhs_time_ranges
            .drain(..)
            .filter(|time_range| {
                (20.0 < (time_range.end - time_range.start))
                    && ((time_range.end - time_range.start) < 180.0)
                    && time_range.end > 0.0
            })
            .collect();
        lhs_time_ranges.sort_by(|a, b| (b.end - b.start).total_cmp(&(a.end - a.start)));
        let mut rhs_time_ranges: Vec<TimeRange> = rhs_time_ranges
            .drain(..)
            .filter(|time_range| {
                (20.0 < (time_range.end - time_range.start))
                    && ((time_range.end - time_range.start) < 180.0)
                    && time_range.end > 0.0
            })
            .collect();
        rhs_time_ranges.sort_by(|a, b| (b.end - b.start).total_cmp(&(a.end - a.start)));
        if lhs_time_ranges.is_empty() || rhs_time_ranges.is_empty() {
            continue;
        }

        all_lhs_time_ranges.push(lhs_time_ranges[0]);
        all_rhs_time_ranges.push(rhs_time_ranges[0]);
    }
    all_lhs_time_ranges.sort_by(|a, b| (a.end - a.start).total_cmp(&(b.end - b.start)));
    all_lhs_time_ranges.reverse();
    all_rhs_time_ranges.sort_by(|a, b| (a.end - a.start).total_cmp(&(b.end - b.start)));
    all_rhs_time_ranges.reverse();

    (all_lhs_time_ranges, all_rhs_time_ranges)
}

fn create_inverse_index(chromaprint: &Vec<u32>) -> HashMap<u32, usize> {
    let mut inverse_index = HashMap::with_capacity(chromaprint.capacity());
    for (i, fingerprint) in chromaprint.iter().enumerate().take(chromaprint.capacity()) {
        inverse_index.insert(*fingerprint, i);
    }
    inverse_index
}

fn find_time_ranges(
    lhs_chromaprint: &[u32],
    rhs_chromaprint: &[u32],
    shift_amount: i32,
    sync_tolerance: u32,
) -> Option<(Vec<TimeRange>, Vec<TimeRange>)> {
    let mut lhs_shift: i32 = 0;
    let mut rhs_shift: i32 = 0;
    if shift_amount < 0 {
        lhs_shift -= shift_amount;
    } else {
        rhs_shift += shift_amount;
    }

    let mut lhs_matching_timestamps = vec![];
    let mut rhs_matching_timestamps = vec![];
    let upper_limit =
        cmp::min(lhs_chromaprint.len(), rhs_chromaprint.len()) as i32 - shift_amount.abs();

    for i in 0..upper_limit {
        let lhs_position = i + lhs_shift;
        let rhs_position = i + rhs_shift;
        let difference = (lhs_chromaprint[lhs_position as usize]
            ^ rhs_chromaprint[rhs_position as usize])
            .count_ones();

        if difference > sync_tolerance {
            continue;
        }

        lhs_matching_timestamps.push(lhs_position as f64 * 0.128);
        rhs_matching_timestamps.push(rhs_position as f64 * 0.128);
    }
    lhs_matching_timestamps.push(f64::MAX);
    rhs_matching_timestamps.push(f64::MAX);

    let lhs_time_ranges = timestamps_to_ranges(lhs_matching_timestamps);
    lhs_time_ranges.as_ref()?;
    let lhs_time_ranges = lhs_time_ranges.unwrap();
    let rhs_time_ranges = timestamps_to_ranges(rhs_matching_timestamps).unwrap();

    Some((lhs_time_ranges, rhs_time_ranges))
}

fn timestamps_to_ranges(mut timestamps: Vec<f64>) -> Option<Vec<TimeRange>> {
    if timestamps.is_empty() {
        return None;
    }

    timestamps.sort_by(|a, b| a.total_cmp(b));

    let mut time_ranges = vec![];
    let mut current_range = TimeRange {
        start: timestamps[0],
        end: timestamps[0],
    };

    for i in 0..timestamps.len() - 1 {
        let current = timestamps[i];
        let next = timestamps[i + 1];
        if next - current <= 1.0 {
            current_range.end = next;
            continue;
        }

        time_ranges.push(current_range);
        current_range.start = next;
        current_range.end = next;
    }
    if !time_ranges.is_empty() {
        Some(time_ranges)
    } else {
        None
    }
}

fn as_u32_le(array: &[u8; 4]) -> u32 {
    #![allow(arithmetic_overflow)]
    (array[0] as u32)
        | ((array[1] as u32) << 8)
        | ((array[2] as u32) << 16)
        | ((array[3] as u32) << 24)
}
