use log::info;

pub struct ProgressHandler;

impl Drop for ProgressHandler {
    fn drop(&mut self) {
        info!(target: "progress_end", "")
    }
}

macro_rules! progress {
    ($($arg:tt)+) => {
        {
            log::info!(target: "progress", $($arg)+);
            $crate::utils::log::ProgressHandler{}
        }
    }
}
pub(crate) use progress;
