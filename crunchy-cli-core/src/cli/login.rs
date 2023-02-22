use crate::utils::context::Context;
use crate::Execute;
use anyhow::bail;
use anyhow::Result;
use crunchyroll_rs::crunchyroll::SessionToken;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, clap::Parser)]
#[clap(about = "Save your login credentials persistent on disk")]
pub struct Login {
    #[arg(help = "Remove your stored credentials (instead of save them)")]
    #[arg(long)]
    pub remove: bool,
}

#[async_trait::async_trait(?Send)]
impl Execute for Login {
    async fn execute(self, ctx: Context) -> Result<()> {
        if let Some(login_file_path) = login_file_path() {
            match ctx.crunchy.session_token().await {
                SessionToken::RefreshToken(refresh_token) => Ok(fs::write(
                    login_file_path,
                    format!("refresh_token:{}", refresh_token),
                )?),
                SessionToken::EtpRt(etp_rt) => {
                    Ok(fs::write(login_file_path, format!("etp_rt:{}", etp_rt))?)
                }
                SessionToken::Anonymous => bail!("Anonymous login cannot be saved"),
            }
        } else {
            bail!("Cannot find config path")
        }
    }
}

pub fn login_file_path() -> Option<PathBuf> {
    dirs::config_dir().map(|config_dir| config_dir.join("crunchy-labs").join("session"))
}
