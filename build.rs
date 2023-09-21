use clap::{Command, CommandFactory};
use clap_complete::shells;
use std::path::{Path, PathBuf};

fn main() -> std::io::Result<()> {
    let rustls_tls = cfg!(feature = "rustls-tls");
    let native_tls = cfg!(feature = "native-tls");
    let openssl_tls = cfg!(any(feature = "openssl-tls", feature = "openssl-tls-static"));

    if rustls_tls as u8 + native_tls as u8 + openssl_tls as u8 > 1 {
        let active_tls_backend = if openssl_tls {
            "openssl"
        } else if native_tls {
            "native tls"
        } else {
            "rustls"
        };

        println!("cargo:warning=Multiple tls backends are activated (through the '*-tls' features). Consider to activate only one as it is not possible to change the backend during runtime. The active backend for this build will be '{}'.", active_tls_backend)
    }

    if cfg!(feature = "openssl") {
        println!("cargo:warning=The 'openssl' feature is deprecated and will be removed in a future version. Use the 'openssl-tls' feature instead.")
    }
    if cfg!(feature = "openssl-static") {
        println!("cargo:warning=The 'openssl-static' feature is deprecated and will be removed in a future version. Use the 'openssl-tls-static' feature instead.")
    }

    // note that we're using an anti-pattern here / violate the rust conventions. build script are
    // not supposed to write outside of 'OUT_DIR'. to have the generated files in the build "root"
    // (the same directory where the output binary lives) is much simpler than in 'OUT_DIR' since
    // its nested in sub directories and is difficult to find (at least more difficult than in the
    // build root)
    let unconventional_out_dir =
        std::path::PathBuf::from(std::env::var_os("OUT_DIR").ok_or(std::io::ErrorKind::NotFound)?)
            .parent()
            .unwrap()
            .parent()
            .unwrap()
            .parent()
            .unwrap()
            .to_path_buf();

    let completions_dir = exist_or_create_dir(unconventional_out_dir.join("completions"))?;
    let manpage_dir = exist_or_create_dir(unconventional_out_dir.join("manpages"))?;

    generate_completions(completions_dir)?;
    generate_manpages(manpage_dir)?;

    Ok(())
}

fn exist_or_create_dir(path: PathBuf) -> std::io::Result<PathBuf> {
    if !path.exists() {
        std::fs::create_dir(path.clone())?
    }
    Ok(path)
}

fn generate_completions(out_dir: PathBuf) -> std::io::Result<()> {
    let mut command: Command = crunchy_cli_core::Cli::command();

    clap_complete::generate_to(
        shells::Bash,
        &mut command.clone(),
        "crunchy-cli",
        out_dir.clone(),
    )?;
    clap_complete::generate_to(
        shells::Elvish,
        &mut command.clone(),
        "crunchy-cli",
        out_dir.clone(),
    )?;
    println!(
        "{}",
        clap_complete::generate_to(
            shells::Fish,
            &mut command.clone(),
            "crunchy-cli",
            out_dir.clone(),
        )?
        .to_string_lossy()
    );
    clap_complete::generate_to(
        shells::PowerShell,
        &mut command.clone(),
        "crunchy-cli",
        out_dir.clone(),
    )?;
    clap_complete::generate_to(shells::Zsh, &mut command, "crunchy-cli", out_dir)?;

    Ok(())
}

fn generate_manpages(out_dir: PathBuf) -> std::io::Result<()> {
    fn generate_command_manpage(
        mut command: Command,
        base_path: &Path,
        sub_name: &str,
    ) -> std::io::Result<()> {
        let (file_name, title) = if sub_name.is_empty() {
            command = command.name("crunchy-cli");
            ("crunchy-cli.1".to_string(), "crunchy-cli".to_string())
        } else {
            command = command.name(format!("crunchy-cli {}", sub_name));
            (
                format!("crunchy-cli-{}.1", sub_name),
                format!("crunchy-cli-{}", sub_name),
            )
        };

        let mut command_buf = vec![];
        let man = clap_mangen::Man::new(command)
            .title(title)
            .date(chrono::Utc::now().format("%b %d, %Y").to_string());
        man.render(&mut command_buf)?;

        std::fs::write(base_path.join(file_name), command_buf)
    }

    generate_command_manpage(crunchy_cli_core::Cli::command(), &out_dir, "")?;
    generate_command_manpage(crunchy_cli_core::Archive::command(), &out_dir, "archive")?;
    generate_command_manpage(crunchy_cli_core::Download::command(), &out_dir, "download")?;
    generate_command_manpage(crunchy_cli_core::Login::command(), &out_dir, "login")?;
    generate_command_manpage(crunchy_cli_core::Search::command(), &out_dir, "search")?;

    Ok(())
}
