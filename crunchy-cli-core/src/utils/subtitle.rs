use crate::utils::os::tempfile;
use anyhow::Result;
use chrono::NaiveTime;
use crunchyroll_rs::media::StreamSubtitle;
use crunchyroll_rs::Locale;
use regex::Regex;
use std::io::Write;
use tempfile::TempPath;

#[derive(Clone)]
pub struct Subtitle {
    pub stream_subtitle: StreamSubtitle,
    pub audio_locale: Locale,
    pub episode_id: String,
    pub forced: bool,
    pub primary: bool,
}

pub async fn download_subtitle(
    subtitle: StreamSubtitle,
    max_length: NaiveTime,
) -> Result<TempPath> {
    let tempfile = tempfile(".ass")?;
    let (mut file, path) = tempfile.into_parts();

    let mut buf = vec![];
    subtitle.write_to(&mut buf).await?;
    buf = fix_subtitle_look_and_feel(buf);
    buf = fix_subtitle_length(buf, max_length);

    file.write_all(buf.as_slice())?;

    Ok(path)
}

/// Add `ScaledBorderAndShadows: yes` to subtitles; without it they look very messy on some video
/// players. See [crunchy-labs/crunchy-cli#66](https://github.com/crunchy-labs/crunchy-cli/issues/66)
/// for more information.
fn fix_subtitle_look_and_feel(raw: Vec<u8>) -> Vec<u8> {
    let mut script_info = false;
    let mut new = String::new();

    for line in String::from_utf8_lossy(raw.as_slice()).split('\n') {
        if line.trim().starts_with('[') && script_info {
            new.push_str("ScaledBorderAndShadow: yes\n");
            script_info = false
        } else if line.trim() == "[Script Info]" {
            script_info = true
        }
        new.push_str(line);
        new.push('\n')
    }

    new.into_bytes()
}

/// Fix the length of subtitles to a specified maximum amount. This is required because sometimes
/// subtitles have an unnecessary entry long after the actual video ends with artificially extends
/// the video length on some video players. To prevent this, the video length must be hard set. See
/// [crunchy-labs/crunchy-cli#32](https://github.com/crunchy-labs/crunchy-cli/issues/32) for more
/// information.
fn fix_subtitle_length(raw: Vec<u8>, max_length: NaiveTime) -> Vec<u8> {
    let re =
        Regex::new(r#"^Dialogue:\s\d+,(?P<start>\d+:\d+:\d+\.\d+),(?P<end>\d+:\d+:\d+\.\d+),"#)
            .unwrap();

    // chrono panics if we try to format NaiveTime with `%2f` and the nano seconds has more than 2
    // digits so them have to be reduced manually to avoid the panic
    fn format_naive_time(native_time: NaiveTime) -> String {
        let formatted_time = native_time.format("%f").to_string();
        format!(
            "{}.{}",
            native_time.format("%T"),
            if formatted_time.len() <= 2 {
                native_time.format("%2f").to_string()
            } else {
                formatted_time.split_at(2).0.parse().unwrap()
            }
        )
    }

    let length_as_string = format_naive_time(max_length);
    let mut new = String::new();

    for line in String::from_utf8_lossy(raw.as_slice()).split('\n') {
        if let Some(capture) = re.captures(line) {
            let start = capture.name("start").map_or(NaiveTime::default(), |s| {
                NaiveTime::parse_from_str(s.as_str(), "%H:%M:%S.%f").unwrap()
            });
            let end = capture.name("end").map_or(NaiveTime::default(), |s| {
                NaiveTime::parse_from_str(s.as_str(), "%H:%M:%S.%f").unwrap()
            });

            if start > max_length {
                continue;
            } else if end > max_length {
                new.push_str(
                    re.replace(
                        line,
                        format!(
                            "Dialogue: {},{},",
                            format_naive_time(start),
                            &length_as_string
                        ),
                    )
                    .to_string()
                    .as_str(),
                )
            } else {
                new.push_str(line)
            }
        } else {
            new.push_str(line)
        }
        new.push('\n')
    }

    new.into_bytes()
}
