#[cfg(not(any(
    feature = "rustls-tls",
    feature = "native-tls",
    feature = "openssl-tls",
    feature = "openssl-tls-static"
)))]
compile_error!("At least one tls feature must be activated");

#[tokio::main]
async fn main() {
    crunchy_cli_core::cli_entrypoint().await
}
