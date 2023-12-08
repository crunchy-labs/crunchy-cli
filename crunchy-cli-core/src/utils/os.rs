use log::debug;
use regex::{Regex, RegexBuilder};
use std::borrow::Cow;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::process::{Command, Stdio};
use std::task::{Context, Poll};
use std::{env, io};
use tempfile::{Builder, NamedTempFile};
use tokio::io::{AsyncRead, ReadBuf};

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

/// Get the temp directory either by the specified `CRUNCHY_CLI_TEMP_DIR` env variable or the dir
/// provided by the os.
pub fn temp_directory() -> PathBuf {
    env::var("CRUNCHY_CLI_TEMP_DIR").map_or(env::temp_dir(), PathBuf::from)
}

/// Any tempfile should be created with this function. The prefix and directory of every file
/// created with this method stays the same which is helpful to query all existing tempfiles and
/// e.g. remove them in a case of ctrl-c. Having one function also good to prevent mistakes like
/// setting the wrong prefix if done manually.
pub fn tempfile<S: AsRef<str>>(suffix: S) -> io::Result<NamedTempFile> {
    let tempfile = Builder::default()
        .prefix(".crunchy-cli_")
        .suffix(suffix.as_ref())
        .tempfile_in(temp_directory())?;
    debug!(
        "Created temporary file: {}",
        tempfile.path().to_string_lossy()
    );
    Ok(tempfile)
}

pub struct TempNamedPipe {
    name: String,

    #[cfg(not(target_os = "windows"))]
    reader: tokio::net::unix::pipe::Receiver,
    #[cfg(target_os = "windows")]
    reader: tokio::net::windows::named_pipe::NamedPipeServer,
}

impl TempNamedPipe {
    pub fn name(&self) -> String {
        self.name.clone()
    }
}

impl AsyncRead for TempNamedPipe {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<io::Result<()>> {
        Pin::new(&mut self.reader).poll_read(cx, buf)
    }
}

impl Drop for TempNamedPipe {
    fn drop(&mut self) {
        #[cfg(not(target_os = "windows"))]
        let _ = nix::unistd::unlink(self.name.as_str());
    }
}

pub fn temp_named_pipe() -> io::Result<TempNamedPipe> {
    let (_, path) = tempfile("")?.keep()?;
    let path = path.to_string_lossy().to_string();
    let _ = std::fs::remove_file(path.clone());

    #[cfg(not(target_os = "windows"))]
    {
        nix::unistd::mkfifo(path.as_str(), nix::sys::stat::Mode::S_IRWXU)?;

        Ok(TempNamedPipe {
            reader: tokio::net::unix::pipe::OpenOptions::new().open_receiver(&path)?,
            name: path,
        })
    }
    #[cfg(target_os = "windows")]
    {
        let path = format!(r"\\.\pipe\{}", &path);

        Ok(TempNamedPipe {
            reader: tokio::net::windows::named_pipe::ServerOptions::new().create(&path)?,
            name: path,
        })
    }
}

/// Check if the given path exists and rename it until the new (renamed) file does not exist.
pub fn free_file(mut path: PathBuf) -> (PathBuf, bool) {
    // do not rename it if it exists but is a special file
    if is_special_file(&path) {
        return (path, false);
    }

    let mut i = 0;
    while path.exists() {
        i += 1;

        let mut ext = path.extension().unwrap_or_default().to_str().unwrap();
        let mut filename = path.file_stem().unwrap_or_default().to_str().unwrap();

        // if the extension is empty, the filename without extension is probably empty
        // (e.g. `.mp4`). in this case Rust assumes that `.mp4` is the file stem rather than the
        // extension. if this is the case, set the extension to the file stem and make the file stem
        // empty
        if ext.is_empty() {
            ext = filename;
            filename = "";
        }

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

lazy_static::lazy_static! {
    static ref ILLEGAL_RE: Regex = Regex::new(r#"[\?<>:\*\|":]"#).unwrap();
    static ref CONTROL_RE: Regex = Regex::new(r"[\x00-\x1f\x80-\x9f]").unwrap();
    static ref RESERVED_RE: Regex = Regex::new(r"^\.+$").unwrap();
    static ref WINDOWS_RESERVED_RE: Regex = RegexBuilder::new(r"(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\..*)?$")
        .case_insensitive(true)
        .build()
        .unwrap();
    static ref WINDOWS_TRAILING_RE: Regex = Regex::new(r"[\. ]+$").unwrap();
}

/// Sanitizes a filename with the option to include/exclude the path separator from sanitizing. This
/// is based of the implementation of the
/// [`sanitize-filename`](https://crates.io/crates/sanitize-filename) crate.
pub fn sanitize<S: AsRef<str>>(path: S, include_path_separator: bool) -> String {
    let path = Cow::from(path.as_ref().trim());

    let path = ILLEGAL_RE.replace_all(&path, "");
    let path = CONTROL_RE.replace_all(&path, "");
    let path = RESERVED_RE.replace(&path, "");

    let collect = |name: String| {
        if name.len() > 255 {
            name[..255].to_string()
        } else {
            name
        }
    };

    if cfg!(windows) {
        let path = WINDOWS_RESERVED_RE.replace(&path, "");
        let path = WINDOWS_TRAILING_RE.replace(&path, "");
        let mut path = path.to_string();
        if include_path_separator {
            path = path.replace(['\\', '/'], "");
        }
        collect(path)
    } else {
        let mut path = path.to_string();
        if include_path_separator {
            path = path.replace('/', "");
        }
        collect(path)
    }
}
