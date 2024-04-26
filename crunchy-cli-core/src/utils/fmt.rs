use chrono::TimeDelta;

pub fn format_time_delta(time_delta: &TimeDelta) -> String {
    let negative = *time_delta < TimeDelta::zero();
    let time_delta = time_delta.abs();
    let hours = time_delta.num_hours();
    let minutes = time_delta.num_minutes() - time_delta.num_hours() * 60;
    let seconds = time_delta.num_seconds() - time_delta.num_minutes() * 60;
    let milliseconds = time_delta.num_milliseconds() - time_delta.num_seconds() * 1000;

    format!(
        "{}{}:{:0>2}:{:0>2}.{:0>3}",
        if negative { "-" } else { "" },
        hours,
        minutes,
        seconds,
        milliseconds
    )
}
