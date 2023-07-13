use anyhow::{bail, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
#[serde(tag = "method")]
pub enum Auth {
    RefreshToken { token: String },
    EtpRt { token: String },
    Anonymous,
}

#[derive(Default, Deserialize, Serialize)]
pub struct Config {
    pub auth: Option<Auth>,
}

impl Config {
    pub fn load() -> Result<Option<Self>> {
        let path = Config::assert_config_file_path(true)?;

        if let Some(p) = path {
            if p.exists() {
                let content = fs::read_to_string(p)?;
                return Ok(Some(toml::from_str(&content)?));
            }
        }
        Ok(None)
    }

    pub fn write(&self) -> Result<()> {
        let path = Config::assert_config_file_path(false)?.unwrap();
        Ok(fs::write(path, toml::to_string(self)?)?)
    }

    pub fn config_file_path() -> Option<PathBuf> {
        dirs::config_dir().map(|config_dir| config_dir.join("crunchy-cli.conf"))
    }

    fn assert_config_file_path(ignore_non_existing_config_dir: bool) -> Result<Option<PathBuf>> {
        let Some(path) = Config::config_file_path() else {
            if ignore_non_existing_config_dir {
                return Ok(None)
            }
            bail!("Cannot find config directory")
        };

        if path.exists() && path.is_dir() {
            bail!(
                "Config path ({}) is a directory (must be a normal file)",
                path.to_string_lossy()
            )
        }

        Ok(Some(path))
    }
}
