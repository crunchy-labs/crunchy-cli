use crate::cli::log::CliLogger;
use crate::utils::context::Context;
use crate::utils::locale::system_locale;
use crate::utils::log::progress;
use anyhow::bail;
use anyhow::Result;
use clap::{Parser, Subcommand};
use crunchyroll_rs::{Crunchyroll, Locale};
use log::{debug, error, info, LevelFilter};
use std::{env, fs};

mod cli;
mod utils;

pub use cli::{archive::Archive, download::Download, login::Login};

#[async_trait::async_trait(?Send)]
trait Execute {
    fn pre_check(&self) -> Result<()> {
        Ok(())
    }
    async fn execute(self, ctx: Context) -> Result<()>;
}

#[derive(Debug, Parser)]
#[clap(author, version = version(), about)]
#[clap(name = "crunchy-cli")]
pub struct Cli {
    #[clap(flatten)]
    verbosity: Option<Verbosity>,

    #[arg(
        help = "Overwrite the language in which results are returned. Default is your system language"
    )]
    #[arg(long)]
    lang: Option<Locale>,

    #[clap(flatten)]
    login_method: LoginMethod,

    #[clap(subcommand)]
    command: Command,
}

fn version() -> String {
    let package_version = env!("CARGO_PKG_VERSION");
    let git_commit_hash = env!("GIT_HASH");
    let build_date = env!("BUILD_DATE");

    if git_commit_hash.is_empty() {
        format!("{}", package_version)
    } else {
        format!("{} ({} {})", package_version, git_commit_hash, build_date)
    }
}

#[derive(Debug, Subcommand)]
enum Command {
    Archive(Archive),
    Download(Download),
    Login(Login),
}

#[derive(Debug, Parser)]
struct Verbosity {
    #[arg(help = "Verbose output")]
    #[arg(short)]
    v: bool,

    #[arg(help = "Very verbose output. Generally not recommended, use '-v' instead")]
    #[arg(long)]
    vv: bool,

    #[arg(help = "Quiet output. Does not print anything unless it's a error")]
    #[arg(
        long_help = "Quiet output. Does not print anything unless it's a error. Can be helpful if you pipe the output to stdout"
    )]
    #[arg(short)]
    q: bool,
}

#[derive(Debug, Parser)]
struct LoginMethod {
    #[arg(help = "Login with credentials (username or email and password)")]
    #[arg(
        long_help = "Login with credentials (username or email and password). Must be provided as user:password"
    )]
    #[arg(long)]
    credentials: Option<String>,
    #[arg(help = "Login with the etp-rt cookie")]
    #[arg(
        long_help = "Login with the etp-rt cookie. This can be obtained when you login on crunchyroll.com and extract it from there"
    )]
    #[arg(long)]
    etp_rt: Option<String>,
}

pub async fn cli_entrypoint() {
    let cli: Cli = Cli::parse();

    if let Some(verbosity) = &cli.verbosity {
        if verbosity.v as u8 + verbosity.q as u8 + verbosity.vv as u8 > 1 {
            eprintln!("Output cannot be verbose ('-v') and quiet ('-q') at the same time");
            std::process::exit(1)
        } else if verbosity.v {
            CliLogger::init(false, LevelFilter::Debug).unwrap()
        } else if verbosity.q {
            CliLogger::init(false, LevelFilter::Error).unwrap()
        } else if verbosity.vv {
            CliLogger::init(true, LevelFilter::Debug).unwrap()
        }
    } else {
        CliLogger::init(false, LevelFilter::Info).unwrap()
    }

    debug!("cli input: {:?}", cli);

    let ctx = match create_ctx(&cli).await {
        Ok(ctx) => ctx,
        Err(e) => {
            error!("{}", e);
            std::process::exit(1)
        }
    };
    debug!("Created context");

    ctrlc::set_handler(move || {
        debug!("Ctrl-c detected");
        if let Ok(dir) = fs::read_dir(&env::temp_dir()) {
            for file in dir.flatten() {
                if file
                    .path()
                    .file_name()
                    .unwrap_or_default()
                    .to_str()
                    .unwrap_or_default()
                    .starts_with(".crunchy-cli_")
                {
                    let result = fs::remove_file(file.path());
                    debug!(
                        "Ctrl-c removed temporary file {} {}",
                        file.path().to_string_lossy(),
                        if result.is_ok() {
                            "successfully"
                        } else {
                            "not successfully"
                        }
                    )
                }
            }
        }
        std::process::exit(1)
    })
    .unwrap();
    debug!("Created ctrl-c handler");

    let result = match cli.command {
        Command::Archive(archive) => archive.execute(ctx).await,
        Command::Download(download) => download.execute(ctx).await,
        Command::Login(login) => {
            if login.remove {
                Ok(())
            } else {
                login.execute(ctx).await
            }
        }
    };
    if let Err(err) = result {
        error!("a unexpected error occurred: {}", err);
        std::process::exit(1)
    }
}

async fn create_ctx(cli: &Cli) -> Result<Context> {
    let crunchy = crunchyroll_session(cli).await?;
    Ok(Context { crunchy })
}

async fn crunchyroll_session(cli: &Cli) -> Result<Crunchyroll> {
    let mut builder = Crunchyroll::builder();
    builder.locale(cli.lang.clone().unwrap_or_else(system_locale));

    let _progress_handler = progress!("Logging in");
    if cli.login_method.credentials.is_none() && cli.login_method.etp_rt.is_none() {
        if let Some(login_file_path) = cli::login::login_file_path() {
            if login_file_path.exists() {
                let session = fs::read_to_string(login_file_path)?;
                if let Some((token_type, token)) = session.split_once(':') {
                    match token_type {
                        "refresh_token" => {
                            return Ok(builder.login_with_refresh_token(token).await?)
                        }
                        "etp_rt" => return Ok(builder.login_with_etp_rt(token).await?),
                        _ => (),
                    }
                }
                bail!("Could not read stored session ('{}')", session)
            }
        }
        bail!("Please use a login method ('--credentials' or '--etp_rt')")
    } else if cli.login_method.credentials.is_some() && cli.login_method.etp_rt.is_some() {
        bail!("Please use only one login method ('--credentials' or '--etp_rt')")
    }

    let crunchy = if let Some(credentials) = &cli.login_method.credentials {
        if let Some((user, password)) = credentials.split_once(':') {
            builder.login_with_credentials(user, password).await?
        } else {
            bail!("Invalid credentials format. Please provide your credentials as user:password")
        }
    } else if let Some(etp_rt) = &cli.login_method.etp_rt {
        builder.login_with_etp_rt(etp_rt).await?
    } else {
        bail!("should never happen")
    };

    info!("Logged in");

    Ok(crunchy)
}
