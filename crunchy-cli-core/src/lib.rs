use crate::utils::context::Context;
use crate::utils::locale::system_locale;
use crate::utils::log::{progress, CliLogger};
use anyhow::bail;
use anyhow::Result;
use clap::{Parser, Subcommand};
use crunchyroll_rs::crunchyroll::CrunchyrollBuilder;
use crunchyroll_rs::error::Error;
use crunchyroll_rs::{Crunchyroll, Locale};
use log::{debug, error, warn, LevelFilter};
use reqwest::Proxy;
use std::{env, fs};

mod archive;
mod download;
mod login;
mod search;
mod utils;

pub use archive::Archive;
use dialoguer::console::Term;
pub use download::Download;
pub use login::Login;
pub use search::Search;

#[async_trait::async_trait(?Send)]
trait Execute {
    fn pre_check(&mut self) -> Result<()> {
        Ok(())
    }
    async fn execute(mut self, ctx: Context) -> Result<()>;
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

    #[arg(help = "Enable experimental fixes which may resolve some unexpected errors")]
    #[arg(
        long_help = "Enable experimental fixes which may resolve some unexpected errors. \
            If everything works as intended this option isn't needed, but sometimes Crunchyroll mislabels \
            the audio of a series/season or episode or returns a wrong season number. This is when using this option might help to solve the issue"
    )]
    #[arg(long, default_value_t = false)]
    experimental_fixes: bool,

    #[clap(flatten)]
    login_method: login::LoginMethod,

    #[arg(help = "Use a proxy to route all traffic through")]
    #[arg(long_help = "Use a proxy to route all traffic through. \
            Make sure that the proxy can either forward TLS requests, which is needed to bypass the (cloudflare) bot protection, or that it is configured so that the proxy can bypass the protection itself")]
    #[clap(long)]
    #[arg(value_parser = crate::utils::clap::clap_parse_proxy)]
    proxy: Option<Proxy>,

    #[arg(help = "Use custom user agent")]
    #[clap(long)]
    user_agent: Option<String>,

    #[clap(subcommand)]
    command: Command,
}

fn version() -> String {
    let package_version = env!("CARGO_PKG_VERSION");
    let git_commit_hash = env!("GIT_HASH");
    let build_date = env!("BUILD_DATE");

    if git_commit_hash.is_empty() {
        package_version.to_string()
    } else {
        format!("{} ({} {})", package_version, git_commit_hash, build_date)
    }
}

#[derive(Debug, Subcommand)]
enum Command {
    Archive(Archive),
    Download(Download),
    Login(Login),
    Search(Search),
}

#[derive(Debug, Parser)]
struct Verbosity {
    #[arg(help = "Verbose output")]
    #[arg(short, long)]
    verbose: bool,

    #[arg(help = "Quiet output. Does not print anything unless it's a error")]
    #[arg(
        long_help = "Quiet output. Does not print anything unless it's a error. Can be helpful if you pipe the output to stdout"
    )]
    #[arg(short, long)]
    quiet: bool,
}

pub async fn cli_entrypoint() {
    let mut cli: Cli = Cli::parse();

    if let Some(verbosity) = &cli.verbosity {
        if verbosity.verbose as u8 + verbosity.quiet as u8 > 1 {
            eprintln!("Output cannot be verbose ('-v') and quiet ('-q') at the same time");
            std::process::exit(1)
        } else if verbosity.verbose {
            CliLogger::init(LevelFilter::Debug).unwrap()
        } else if verbosity.quiet {
            CliLogger::init(LevelFilter::Error).unwrap()
        }
    } else {
        CliLogger::init(LevelFilter::Info).unwrap()
    }

    debug!("cli input: {:?}", cli);

    match &mut cli.command {
        Command::Archive(archive) => {
            // prevent interactive select to be shown when output should be quiet
            if cli.verbosity.is_some() && cli.verbosity.as_ref().unwrap().quiet {
                archive.yes = true;
            }
            pre_check_executor(archive).await
        }
        Command::Download(download) => {
            // prevent interactive select to be shown when output should be quiet
            if cli.verbosity.is_some() && cli.verbosity.as_ref().unwrap().quiet {
                download.yes = true;
            }
            pre_check_executor(download).await
        }
        Command::Login(login) => {
            if login.remove {
                if let Some(session_file) = login::session_file_path() {
                    let _ = fs::remove_file(session_file);
                }
                return;
            } else {
                pre_check_executor(login).await
            }
        }
        Command::Search(search) => pre_check_executor(search).await,
    };

    let ctx = match create_ctx(&mut cli).await {
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
        // when pressing ctrl-c while interactively choosing seasons the cursor stays hidden, this
        // line shows it again
        let _ = Term::stdout().show_cursor();
        std::process::exit(1)
    })
    .unwrap();
    debug!("Created ctrl-c handler");

    match cli.command {
        Command::Archive(archive) => execute_executor(archive, ctx).await,
        Command::Download(download) => execute_executor(download, ctx).await,
        Command::Login(login) => execute_executor(login, ctx).await,
        Command::Search(search) => execute_executor(search, ctx).await,
    };
}

async fn pre_check_executor(executor: &mut impl Execute) {
    if let Err(err) = executor.pre_check() {
        error!("Misconfigurations detected: {}", err);
        std::process::exit(1)
    }
}

async fn execute_executor(executor: impl Execute, ctx: Context) {
    if let Err(mut err) = executor.execute(ctx).await {
        if let Some(crunchy_error) = err.downcast_mut::<Error>() {
            if let Error::Block { message, .. } = crunchy_error {
                *message = "Triggered Cloudflare bot protection. Try again later or use a VPN or proxy to spoof your location".to_string()
            } else if let Error::Request { message, .. } = crunchy_error {
                *message = "You've probably hit a rate limit. Try again later, generally after 10-20 minutes the rate limit is over and you can continue to use the cli".to_string()
            }

            error!("An error occurred: {}", crunchy_error)
        } else {
            error!("An error occurred: {}", err)
        }

        std::process::exit(1)
    }
}

async fn create_ctx(cli: &mut Cli) -> Result<Context> {
    let crunchy = crunchyroll_session(cli).await?;
    Ok(Context { crunchy })
}

async fn crunchyroll_session(cli: &mut Cli) -> Result<Crunchyroll> {
    let supported_langs = vec![
        Locale::ar_ME,
        Locale::de_DE,
        Locale::en_US,
        Locale::es_ES,
        Locale::es_419,
        Locale::fr_FR,
        Locale::it_IT,
        Locale::pt_BR,
        Locale::pt_PT,
        Locale::ru_RU,
    ];
    let locale = if let Some(lang) = &cli.lang {
        if !supported_langs.contains(lang) {
            bail!(
                "Via `--lang` specified language is not supported. Supported languages: {}",
                supported_langs
                    .iter()
                    .map(|l| format!("`{}` ({})", l, l.to_human_readable()))
                    .collect::<Vec<String>>()
                    .join(", ")
            )
        }
        lang.clone()
    } else {
        let mut lang = system_locale();
        if !supported_langs.contains(&lang) {
            warn!("Recognized system locale is not supported. Using en-US as default. Use `--lang` to overwrite the used language");
            lang = Locale::en_US
        }
        lang
    };

    let mut builder = Crunchyroll::builder()
        .locale(locale)
        .client({
            let mut builder = CrunchyrollBuilder::predefined_client_builder();
            if let Some(p) = &cli.proxy {
                builder = builder.proxy(p.clone())
            }
            if let Some(ua) = &cli.user_agent {
                builder = builder.user_agent(ua)
            }

            #[cfg(any(feature = "openssl-tls", feature = "openssl-tls-static"))]
            let client = {
                let mut builder = builder.use_native_tls().tls_built_in_root_certs(false);

                for certificate in rustls_native_certs::load_native_certs().unwrap() {
                    builder = builder.add_root_certificate(
                        reqwest::Certificate::from_der(certificate.0.as_slice()).unwrap(),
                    )
                }

                builder.build().unwrap()
            };
            #[cfg(not(any(feature = "openssl-tls", feature = "openssl-tls-static")))]
            let client = builder.build().unwrap();

            client
        })
        .stabilization_locales(cli.experimental_fixes)
        .stabilization_season_number(cli.experimental_fixes);
    if let Command::Download(download) = &cli.command {
        builder = builder.preferred_audio_locale(download.audio.clone())
    }

    let root_login_methods_count = cli.login_method.credentials.is_some() as u8
        + cli.login_method.etp_rt.is_some() as u8
        + cli.login_method.anonymous as u8;
    let mut login_login_methods_count = 0;
    if let Command::Login(login) = &cli.command {
        login_login_methods_count += login.login_method.credentials.is_some() as u8
            + login.login_method.etp_rt.is_some() as u8
            + login.login_method.anonymous as u8
    }

    let progress_handler = progress!("Logging in");
    if root_login_methods_count + login_login_methods_count == 0 {
        if let Some(login_file_path) = login::session_file_path() {
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
        bail!("Please use a login method ('--credentials', '--etp-rt' or '--anonymous')")
    } else if root_login_methods_count + login_login_methods_count > 1 {
        bail!("Please use only one login method ('--credentials', '--etp-rt' or '--anonymous')")
    }

    let login_method = if login_login_methods_count > 0 {
        if let Command::Login(login) = &cli.command {
            login.login_method.clone()
        } else {
            unreachable!()
        }
    } else {
        cli.login_method.clone()
    };

    let crunchy = if let Some(credentials) = &login_method.credentials {
        if let Some((user, password)) = credentials.split_once(':') {
            builder.login_with_credentials(user, password).await?
        } else {
            bail!("Invalid credentials format. Please provide your credentials as user:password")
        }
    } else if let Some(etp_rt) = &login_method.etp_rt {
        builder.login_with_etp_rt(etp_rt).await?
    } else if login_method.anonymous {
        builder.login_anonymously().await?
    } else {
        bail!("should never happen")
    };

    progress_handler.stop("Logged in");

    Ok(crunchy)
}
