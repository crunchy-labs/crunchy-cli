use log::{
    set_boxed_logger, set_max_level, Level, LevelFilter, Log, Metadata, Record, SetLoggerError,
};
use std::io::{stdout, Write};
use std::sync::{mpsc, Mutex};
use std::thread;
use std::thread::JoinHandle;
use std::time::Duration;

struct CliProgress {
    handler: JoinHandle<()>,
    sender: mpsc::SyncSender<(String, Level)>,
}

impl CliProgress {
    fn new(record: &Record) -> Self {
        let (tx, rx) = mpsc::sync_channel(1);

        let init_message = format!("{}", record.args());
        let init_level = record.level();
        let handler = thread::spawn(move || {
            let states = ["-", "\\", "|", "/"];

            let mut old_message = init_message.clone();
            let mut latest_info_message = init_message;
            let mut old_level = init_level;
            for i in 0.. {
                let (msg, level) = match rx.try_recv() {
                    Ok(payload) => payload,
                    Err(e) => match e {
                        mpsc::TryRecvError::Empty => (old_message.clone(), old_level),
                        mpsc::TryRecvError::Disconnected => break,
                    },
                };

                // clear last line
                // prefix (2), space (1), state (1), space (1), message(n)
                let _ = write!(stdout(), "\r     {}", " ".repeat(old_message.len()));

                if old_level != level || old_message != msg {
                    if old_level <= Level::Warn {
                        let _ = writeln!(stdout(), "\r:: • {}", old_message);
                    } else if old_level == Level::Info && level <= Level::Warn {
                        let _ = writeln!(stdout(), "\r:: → {}", old_message);
                    } else if level == Level::Info {
                        latest_info_message = msg.clone();
                    }
                }

                let _ = write!(
                    stdout(),
                    "\r:: {} {}",
                    states[i / 2 % states.len()],
                    if level == Level::Info {
                        &msg
                    } else {
                        &latest_info_message
                    }
                );
                let _ = stdout().flush();

                old_message = msg;
                old_level = level;

                thread::sleep(Duration::from_millis(100));
            }

            // clear last line
            // prefix (2), space (1), state (1), space (1), message(n)
            let _ = write!(stdout(), "\r     {}", " ".repeat(old_message.len()));
            let _ = writeln!(stdout(), "\r:: ✓ {}", old_message);
            let _ = stdout().flush();
        });

        Self {
            handler,
            sender: tx,
        }
    }

    fn send(&self, record: &Record) {
        let _ = self
            .sender
            .send((format!("{}", record.args()), record.level()));
    }

    fn stop(self) {
        drop(self.sender);
        let _ = self.handler.join();
    }
}

#[allow(clippy::type_complexity)]
pub struct CliLogger {
    level: LevelFilter,
    progress: Mutex<Option<CliProgress>>,
}

impl Log for CliLogger {
    fn enabled(&self, metadata: &Metadata) -> bool {
        metadata.level() <= self.level
    }

    fn log(&self, record: &Record) {
        if !self.enabled(record.metadata())
            || (record.target() != "progress"
                && record.target() != "progress_end"
                && !record.target().starts_with("crunchy_cli"))
        {
            return;
        }

        if self.level >= LevelFilter::Debug {
            self.extended(record);
            return;
        }

        match record.target() {
            "progress" => self.progress(record, false),
            "progress_end" => self.progress(record, true),
            _ => {
                if self.progress.lock().unwrap().is_some() {
                    self.progress(record, false);
                } else if record.level() > Level::Warn {
                    self.normal(record)
                } else {
                    self.error(record)
                }
            }
        }
    }

    fn flush(&self) {
        let _ = stdout().flush();
    }
}

impl CliLogger {
    pub fn new(level: LevelFilter) -> Self {
        Self {
            level,
            progress: Mutex::new(None),
        }
    }

    pub fn init(level: LevelFilter) -> Result<(), SetLoggerError> {
        set_max_level(level);
        set_boxed_logger(Box::new(CliLogger::new(level)))
    }

    fn extended(&self, record: &Record) {
        println!(
            "[{}] {}  {} ({}) {}",
            chrono::Utc::now().format("%Y-%m-%d %H:%M:%S"),
            record.level(),
            // replace the 'progress' prefix if this function is invoked via 'progress!'
            record
                .target()
                .replacen("progress", "crunchy_cli", 1)
                .replacen("progress_end", "crunchy_cli", 1),
            format!("{:?}", thread::current().id())
                .replace("ThreadId(", "")
                .replace(')', ""),
            record.args()
        )
    }

    fn normal(&self, record: &Record) {
        println!(":: {}", record.args())
    }

    fn error(&self, record: &Record) {
        eprintln!(":: {}", record.args())
    }

    fn progress(&self, record: &Record, stop: bool) {
        let mut progress_option = self.progress.lock().unwrap();
        if stop && progress_option.is_some() {
            progress_option.take().unwrap().stop()
        } else if let Some(p) = &*progress_option {
            p.send(record);
        } else {
            *progress_option = Some(CliProgress::new(record))
        }
    }
}

macro_rules! tab_info {
    ($($arg:tt)+) => {
        if log::max_level() == log::LevelFilter::Debug {
            info!($($arg)+)
        } else {
            info!("\t{}", format!($($arg)+))
        }
    }
}
pub(crate) use tab_info;
