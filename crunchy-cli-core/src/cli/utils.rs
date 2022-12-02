use crate::utils::context::Context;
use anyhow::Result;
use crunchyroll_rs::media::{Resolution, VariantData, VariantSegment};
use isahc::AsyncReadResponseExt;
use log::{debug, LevelFilter};
use std::borrow::{Borrow, BorrowMut};
use std::collections::BTreeMap;
use std::io;
use std::io::Write;
use std::sync::{mpsc, Arc, Mutex};
use std::time::Duration;
use tokio::task::JoinSet;

pub fn find_resolution(
    mut streaming_data: Vec<VariantData>,
    resolution: &Resolution,
) -> Option<VariantData> {
    streaming_data.sort_by(|a, b| a.resolution.width.cmp(&b.resolution.width).reverse());
    match resolution.height {
        u64::MAX => Some(streaming_data.into_iter().next().unwrap()),
        u64::MIN => Some(streaming_data.into_iter().last().unwrap()),
        _ => streaming_data
            .into_iter()
            .find(|v| resolution.height == u64::MAX || v.resolution.height == resolution.height),
    }
}

pub async fn download_segments(
    _ctx: &Context,
    writer: &mut impl Write,
    message: Option<String>,
    variant_data: VariantData,
) -> Result<()> {
    let segments = variant_data.segments().await?;
    let total_segments = segments.len();

    let client = Arc::new(variant_data.download_client());
    let count = Arc::new(Mutex::new(0));
    let amount = Arc::new(Mutex::new(0));

    // only print progress when log level is info
    let output_handler = if log::max_level() == LevelFilter::Info {
        let output_count = count.clone();
        let output_amount = amount.clone();
        Some(tokio::spawn(async move {
            let sleep_time_ms = 100;
            let iter_per_sec = 1000f64 / sleep_time_ms as f64;

            let mut bytes_start = 0f64;
            let mut speed = 0f64;
            let mut percentage = 0f64;

            while *output_count.lock().unwrap() < total_segments || percentage < 100f64 {
                let tmp_amount = *output_amount.lock().unwrap() as f64;

                let tmp_speed = (tmp_amount - bytes_start) / 1024f64 / 1024f64;
                if *output_count.lock().unwrap() < 3 {
                    speed = tmp_speed;
                } else {
                    let (old_speed_ratio, new_speed_ratio) = if iter_per_sec <= 1f64 {
                        (0f64, 1f64)
                    } else {
                        (1f64 - (1f64 / iter_per_sec), (1f64 / iter_per_sec))
                    };

                    // calculate the average download speed "smoother"
                    speed = (speed * old_speed_ratio) + (tmp_speed * new_speed_ratio);
                }

                percentage =
                    (*output_count.lock().unwrap() as f64 / total_segments as f64) * 100f64;

                let size = terminal_size::terminal_size()
                    .unwrap_or((terminal_size::Width(60), terminal_size::Height(0)))
                    .0
                     .0 as usize;

                let progress_available = size
                    - if let Some(msg) = &message {
                        35 + msg.len()
                    } else {
                        33
                    };
                let progress_done_count =
                    (progress_available as f64 * (percentage / 100f64)).ceil() as usize;
                let progress_to_do_count = progress_available - progress_done_count;

                let _ = write!(
                    io::stdout(),
                    "\r:: {}{:>5.1} MiB  {:>5.2} MiB/s [{}{}] {:>3}%",
                    message.clone().map_or("".to_string(), |msg| msg + " "),
                    tmp_amount / 1024f64 / 1024f64,
                    speed * iter_per_sec,
                    "#".repeat(progress_done_count),
                    "-".repeat(progress_to_do_count),
                    percentage as usize
                );

                bytes_start = tmp_amount;

                tokio::time::sleep(Duration::from_millis(sleep_time_ms)).await;
            }
            println!()
        }))
    } else {
        None
    };

    let cpus = num_cpus::get();
    let mut segs: Vec<Vec<VariantSegment>> = Vec::with_capacity(cpus);
    for _ in 0..cpus {
        segs.push(vec![])
    }
    for (i, segment) in segments.into_iter().enumerate() {
        segs[i - ((i / cpus) * cpus)].push(segment);
    }

    let (sender, receiver) = mpsc::channel();

    let mut join_set: JoinSet<Result<()>> = JoinSet::new();
    for num in 0..cpus {
        let thread_client = client.clone();
        let thread_sender = sender.clone();
        let thread_segments = segs.remove(0);
        let thread_amount = amount.clone();
        let thread_count = count.clone();
        join_set.spawn(async move {
            for (i, segment) in thread_segments.into_iter().enumerate() {
                let mut response = thread_client.get_async(&segment.url).await?;
                let mut buf = response.bytes().await?.to_vec();

                *thread_amount.lock().unwrap() += buf.len();

                buf = VariantSegment::decrypt(buf.borrow_mut(), segment.key)?.to_vec();
                debug!(
                    "Downloaded and decrypted segment {} ({})",
                    num + (i * cpus),
                    segment.url
                );
                thread_sender.send((num + (i * cpus), buf))?;

                *thread_count.lock().unwrap() += 1;
            }

            Ok(())
        });
    }

    let mut data_pos = 0usize;
    let mut buf: BTreeMap<usize, Vec<u8>> = BTreeMap::new();
    loop {
        // is always `Some` because `sender` does not get dropped when all threads are finished
        let data = receiver.recv().unwrap();

        if data_pos == data.0 {
            writer.write_all(data.1.borrow())?;
            data_pos += 1;
        } else {
            buf.insert(data.0, data.1);
        }
        while let Some(b) = buf.remove(&data_pos) {
            writer.write_all(b.borrow())?;
            data_pos += 1;
        }

        if *count.lock().unwrap() >= total_segments {
            break;
        }
    }

    while let Some(joined) = join_set.join_next().await {
        joined??
    }
    if let Some(handler) = output_handler {
        handler.await?
    }

    Ok(())
}
