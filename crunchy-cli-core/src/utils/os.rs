use log::debug;
use regex::{Regex, RegexBuilder};
use std::borrow::Cow;
use std::io::ErrorKind;
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::process::{Command, Stdio};
use std::task::{Context, Poll};
use std::{env, fs, io};
use tempfile::{Builder, NamedTempFile, TempPath};
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

pub fn cache_dir<S: AsRef<str>>(name: S) -> io::Result<PathBuf> {
    let cache_dir = temp_directory().join(format!(".crunchy-cli_{}_cache", name.as_ref()));
    fs::create_dir_all(&cache_dir)?;
    Ok(cache_dir)
}

pub struct TempNamedPipe {
    path: TempPath,

    #[cfg(not(target_os = "windows"))]
    reader: tokio::net::unix::pipe::Receiver,
    #[cfg(target_os = "windows")]
    file: tokio::fs::File,
}

impl TempNamedPipe {
    pub fn path(&self) -> &Path {
        &self.path
    }
}

impl AsyncRead for TempNamedPipe {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<io::Result<()>> {
        #[cfg(not(target_os = "windows"))]
        return Pin::new(&mut self.reader).poll_read(cx, buf);
        // very very dirty implementation of a 'tail' like behavior
        #[cfg(target_os = "windows")]
        {
            let mut tmp_bytes = vec![0; buf.remaining()];
            let mut tmp_buf = ReadBuf::new(tmp_bytes.as_mut_slice());

            loop {
                return match Pin::new(&mut self.file).poll_read(cx, &mut tmp_buf) {
                    Poll::Ready(r) => {
                        if r.is_ok() {
                            if !tmp_buf.filled().is_empty() {
                                buf.put_slice(tmp_buf.filled())
                            } else {
                                // sleep to not loop insanely fast and consume unnecessary system resources
                                std::thread::sleep(std::time::Duration::from_millis(50));
                                continue;
                            }
                        }
                        Poll::Ready(r)
                    }
                    Poll::Pending => Poll::Pending,
                };
            }
        }
    }
}

impl Drop for TempNamedPipe {
    fn drop(&mut self) {
        #[cfg(not(target_os = "windows"))]
        let _ = nix::unistd::unlink(self.path.to_string_lossy().to_string().as_str());
    }
}

pub fn temp_named_pipe() -> io::Result<TempNamedPipe> {
    let tmp = tempfile("")?;

    #[cfg(not(target_os = "windows"))]
    {
        let path = tmp.into_temp_path();
        let _ = fs::remove_file(&path);

        nix::unistd::mkfifo(
            path.to_string_lossy().to_string().as_str(),
            nix::sys::stat::Mode::S_IRWXU,
        )?;

        Ok(TempNamedPipe {
            reader: tokio::net::unix::pipe::OpenOptions::new().open_receiver(&path)?,
            path,
        })
    }
    #[cfg(target_os = "windows")]
    {
        let (file, path) = tmp.into_parts();

        Ok(TempNamedPipe {
            file: tokio::fs::File::from_std(file),
            path,
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
    static ref WINDOWS_NON_PRINTABLE_RE: Regex = Regex::new(r"[\x00-\x1f\x80-\x9f]").unwrap();
    static ref WINDOWS_ILLEGAL_RE: Regex = Regex::new(r#"[<>:"|?*]"#).unwrap();
    static ref WINDOWS_RESERVED_RE: Regex = RegexBuilder::new(r"(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\..*)?$")
        .case_insensitive(true)
        .build()
        .unwrap();
    static ref WINDOWS_TRAILING_RE: Regex = Regex::new(r"[\. ]+$").unwrap();

    static ref LINUX_NON_PRINTABLE: Regex = Regex::new(r"[\x00]").unwrap();

    static ref RESERVED_RE: Regex = Regex::new(r"^\.+$").unwrap();
}

/// Sanitizes a filename with the option to include/exclude the path separator from sanitizing.
pub fn sanitize<S: AsRef<str>>(path: S, include_path_separator: bool, universal: bool) -> String {
    let path = Cow::from(path.as_ref().trim());

    let path = RESERVED_RE.replace(&path, "");

    let collect = |name: String| {
        if name.len() > 255 {
            name[..255].to_string()
        } else {
            name
        }
    };

    if universal || cfg!(windows) {
        let path = WINDOWS_NON_PRINTABLE_RE.replace_all(&path, "");
        let path = WINDOWS_ILLEGAL_RE.replace_all(&path, "");
        let path = WINDOWS_RESERVED_RE.replace_all(&path, "");
        let path = WINDOWS_TRAILING_RE.replace(&path, "");
        let mut path = path.to_string();
        if include_path_separator {
            path = path.replace(['\\', '/'], "");
        }
        collect(path)
    } else {
        let path = LINUX_NON_PRINTABLE.replace_all(&path, "");
        let mut path = path.to_string();
        if include_path_separator {
            path = path.replace('/', "");
        }
        collect(path)
    }
}
