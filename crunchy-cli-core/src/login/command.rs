use crate::utils::context::Context;
use crate::Execute;
use anyhow::bail;
use anyhow::Result;
use clap::Parser;
use crunchyroll_rs::crunchyroll::SessionToken;
use log::info;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, clap::Parser)]
#[clap(about = "Save your login credentials persistent on disk")]
pub struct Login {
    #[arg(help = "Remove your stored credentials (instead of saving them)")]
    #[arg(long)]
    pub remove: bool,
}

impl Execute for Login {
    async fn execute(self, ctx: Context) -> Result<()> {
        if let Some(login_file_path) = session_file_path() {
            fs::create_dir_all(login_file_path.parent().unwrap())?;

            match ctx.crunchy.session_token().await {
                SessionToken::RefreshToken(refresh_token) => {
                    fs::write(login_file_path, format!("refresh_token:{}", refresh_token))?
                }
                SessionToken::EtpRt(etp_rt) => {
                    fs::write(login_file_path, format!("etp_rt:{}", etp_rt))?
                }
                SessionToken::Anonymous => bail!("Anonymous login cannot be saved"),
            }

            info!("Saved login");

            Ok(())
        } else {
            bail!("Cannot find config path")
        }
    }
}

#[derive(Clone, Debug, Parser)]
pub struct LoginMethod {
    #[arg(
        help = "Login with credentials (email and password). Must be provided as email:password"
    )]
    #[arg(global = true, long)]
    pub credentials: Option<String>,
    #[arg(help = "Login with the etp-rt cookie")]
    #[arg(
        long_help = "Login with the etp-rt cookie. This can be obtained when you login on crunchyroll.com and extract it from there"
    )]
    #[arg(global = true, long)]
    pub etp_rt: Option<String>,
    #[arg(help = "Login anonymously / without an account")]
    #[arg(global = true, long, default_value_t = false)]
    pub anonymous: bool,
}

pub fn session_file_path() -> Option<PathBuf> {
    dirs::config_dir().map(|config_dir| config_dir.join("crunchy-cli").join("session"))
}
