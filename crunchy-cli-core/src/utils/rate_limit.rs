use async_speed_limit::Limiter;
use crunchyroll_rs::error::Error;
use futures_util::TryStreamExt;
use reqwest::{Client, Request, Response, ResponseBuilderExt};
use std::future::Future;
use std::io;
use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};
use tower_service::Service;

#[derive(Clone)]
pub struct RateLimiterService {
    client: Arc<Client>,
    rate_limiter: Limiter,
}

impl RateLimiterService {
    pub fn new(bytes: u32, client: Client) -> Self {
        Self {
            client: Arc::new(client),
            rate_limiter: Limiter::new(bytes as f64),
        }
    }
}

impl Service<Request> for RateLimiterService {
    type Response = Response;
    type Error = Error;
    type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

    fn poll_ready(&mut self, _: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, req: Request) -> Self::Future {
        let client = self.client.clone();
        let rate_limiter = self.rate_limiter.clone();

        Box::pin(async move {
            let mut body = vec![];
            let res = client.execute(req).await?;
            let _url = res.url().clone().to_string();
            let url = _url.as_str();

            let mut http_res = http::Response::builder()
                .url(res.url().clone())
                .status(res.status())
                .version(res.version());
            *http_res.headers_mut().unwrap() = res.headers().clone();
            http_res
                .extensions_ref()
                .unwrap()
                .clone_from(&res.extensions());

            let limiter = rate_limiter.limit(
                res.bytes_stream()
                    .map_err(io::Error::other)
                    .into_async_read(),
            );

            futures_util::io::copy(limiter, &mut body)
                .await
                .map_err(|e| Error::Request {
                    url: url.to_string(),
                    status: None,
                    message: e.to_string(),
                })?;

            Ok(Response::from(http_res.body(body).unwrap()))
        })
    }
}
