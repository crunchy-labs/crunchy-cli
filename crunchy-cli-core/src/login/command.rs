use crate::utils::config::Auth;
use crate::utils::context::Context;
use crate::Execute;
use anyhow::Result;
use clap::Parser;
use crunchyroll_rs::crunchyroll::SessionToken;
use log::info;

#[derive(Debug, clap::Parser)]
#[clap(about = "Save your login credentials persistent on disk")]
pub struct Login {
    #[clap(flatten)]
    pub login_method: LoginMethod,
    #[arg(help = "Remove your stored credentials (instead of saving them)")]
    #[arg(long)]
    pub remove: bool,
}

#[async_trait::async_trait(?Send)]
impl Execute for Login {
    async fn execute(self, mut ctx: Context) -> Result<()> {
        let auth = match ctx.crunchy.session_token().await {
            SessionToken::RefreshToken(token) => Auth::RefreshToken { token },
            SessionToken::EtpRt(token) => Auth::EtpRt { token },
            SessionToken::Anonymous => Auth::Anonymous,
        };
        ctx.config.auth = Some(auth);
        ctx.config.write()?;

        info!("Saved login");

        Ok(())
    }
}

#[derive(Clone, Debug, Parser)]
pub struct LoginMethod {
    #[arg(
        help = "Login with credentials (username or email and password). Must be provided as user:password"
    )]
    #[arg(long)]
    pub credentials: Option<String>,
    #[arg(help = "Login with the etp-rt cookie")]
    #[arg(
        long_help = "Login with the etp-rt cookie. This can be obtained when you login on crunchyroll.com and extract it from there"
    )]
    #[arg(long)]
    pub etp_rt: Option<String>,
    #[arg(help = "Login anonymously / without an account")]
    #[arg(long, default_value_t = false)]
    pub anonymous: bool,
}
