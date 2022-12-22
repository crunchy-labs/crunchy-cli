use log::info;

pub struct ProgressHandler {
    pub(crate) stopped: bool,
}

impl Drop for ProgressHandler {
    fn drop(&mut self) {
        if !self.stopped {
            info!(target: "progress_end", "")
        }
    }
}

impl ProgressHandler {
    pub(crate) fn stop<S: AsRef<str>>(mut self, msg: S) {
        self.stopped = true;
        info!(target: "progress_end", "{}", msg.as_ref())
    }
}

macro_rules! progress {
    ($($arg:tt)+) => {
        {
            log::info!(target: "progress", $($arg)+);
            $crate::utils::log::ProgressHandler{stopped: false}
        }
    }
}
pub(crate) use progress;
