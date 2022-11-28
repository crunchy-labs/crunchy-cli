fn main() -> std::io::Result<()> {
    println!(
        "cargo:rustc-env=GIT_HASH={}",
        get_short_commit_hash()?.unwrap_or_default()
    );
    println!(
        "cargo:rustc-env=BUILD_DATE={}",
        chrono::Utc::now().format("%F")
    );

    Ok(())
}

fn get_short_commit_hash() -> std::io::Result<Option<String>> {
    let git = std::process::Command::new("git")
        .arg("rev-parse")
        .arg("--short")
        .arg("HEAD")
        .output();

    match git {
        Ok(cmd) => Ok(Some(
            String::from_utf8_lossy(cmd.stdout.as_slice()).to_string(),
        )),
        Err(e) => {
            if e.kind() != std::io::ErrorKind::NotFound {
                Err(e)
            } else {
                Ok(None)
            }
        }
    }
}
