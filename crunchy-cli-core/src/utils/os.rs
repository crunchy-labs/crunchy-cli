use log::debug;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::{env, io};
use tempfile::{Builder, NamedTempFile};

pub fn has_ffmpeg() -> bool {
    if let Err(e) = Command::new("ffmpeg").stderr(Stdio::null()).spawn() {
        if ErrorKind::NotFound != e.kind() {
            debug!(
                "unknown error occurred while checking if ffmpeg exists: {}",
                e.kind()
            )
        }
        false
    } else {
        true
    }
}

/// Any tempfile should be created with this function. The prefix and directory of every file
/// created with this method stays the same which is helpful to query all existing tempfiles and
/// e.g. remove them in a case of ctrl-c. Having one function also good to prevent mistakes like
/// setting the wrong prefix if done manually.
pub fn tempfile<S: AsRef<str>>(suffix: S) -> io::Result<NamedTempFile> {
    let tempfile = Builder::default()
        .prefix(".crunchy-cli_")
        .suffix(suffix.as_ref())
        .tempfile_in(&env::temp_dir())?;
    debug!(
        "Created temporary file: {}",
        tempfile.path().to_string_lossy()
    );
    Ok(tempfile)
}

/// Check if the given path exists and rename it until the new (renamed) file does not exist.
pub fn free_file(mut path: PathBuf) -> (PathBuf, bool) {
    // if it's a special file does not rename it
    if is_special_file(&path) {
        return (path, false);
    }

    let mut i = 0;
    while path.exists() {
        i += 1;

        let ext = path.extension().unwrap_or_default().to_string_lossy();
        let mut filename = path.file_stem().unwrap_or_default().to_str().unwrap();

        if filename.ends_with(&format!(" ({})", i - 1)) {
            filename = filename.strip_suffix(&format!(" ({})", i - 1)).unwrap();
        }

        path.set_file_name(format!("{} ({}).{}", filename, i, ext))
    }
    (path, i != 0)
}

/// Check if the given path is a special file. On Linux this is probably a pipe and on Windows
/// ¯\_(ツ)_/¯
pub fn is_special_file<P: AsRef<Path>>(path: P) -> bool {
    path.as_ref().exists() && !path.as_ref().is_file() && !path.as_ref().is_dir()
}
