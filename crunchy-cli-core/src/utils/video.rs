use anyhow::Result;
use chrono::NaiveTime;
use regex::Regex;
use std::path::PathBuf;
use std::process::{Command, Stdio};

/// Get the length of a video. This is required because sometimes subtitles have an unnecessary entry
/// long after the actual video ends with artificially extends the video length on some video players.
/// To prevent this, the video length must be hard set. See
/// [crunchy-labs/crunchy-cli#32](https://github.com/crunchy-labs/crunchy-cli/issues/32) for more
/// information.
pub fn get_video_length(path: PathBuf) -> Result<NaiveTime> {
    let video_length = Regex::new(r"Duration:\s(?P<time>\d+:\d+:\d+\.\d+),")?;

    let ffmpeg = Command::new("ffmpeg")
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .arg("-y")
        .args(["-i", path.to_str().unwrap()])
        .output()?;
    let ffmpeg_output = String::from_utf8(ffmpeg.stderr)?;
    let caps = video_length.captures(ffmpeg_output.as_str()).unwrap();

    Ok(NaiveTime::parse_from_str(caps.name("time").unwrap().as_str(), "%H:%M:%S%.f").unwrap())
}
