// Onboarding talks to the local server on the page's behalf: the page never
// holds the API key, and the agent probe and runs live in the server.
use std::sync::Mutex;
use std::time::Duration;

use reqwest::{Client, Method, RequestBuilder};
use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tauri::{AppHandle, Emitter, Manager};

use crate::commands::ShellState;
use crate::config::LOCAL_ID;
use crate::remote::ServerError;
use crate::servers::local_key;

const HELLO_WAIT: Duration = Duration::from_secs(90);
const HELLO_EVENT: &str = "onboarding-hello";
struct Local {
    base: String,
    key: Option<String>,
}

impl Local {
    fn req(&self, client: &Client, method: Method, path: &str) -> RequestBuilder {
        let req = client.request(method, format!("{}{path}", self.base));
        match &self.key {
            Some(k) => req.header("X-Hylo-API-Key", k),
            None => req,
        }
    }
}

fn client(timeout: Option<Duration>) -> Result<Client, String> {
    let mut b = Client::builder().no_proxy().connect_timeout(Duration::from_secs(3));
    if let Some(t) = timeout {
        b = b.timeout(t);
    }
    b.build().map_err(|e| e.to_string())
}

async fn fetch_json<T: DeserializeOwned>(req: RequestBuilder) -> Result<T, String> {
    req.send()
        .await
        .map_err(|e| e.to_string())?
        .error_for_status()
        .map_err(|e| e.to_string())?
        .json()
        .await
        .map_err(|e| e.to_string())
}

/// The local server, started if needed, and a key it accepts.
async fn ready(app: &AppHandle) -> Result<Local, String> {
    crate::local::ensure(app, true).await.map_err(|e| e.message())?;
    let base = app
        .state::<ShellState>()
        .read_config(|cfg| cfg.find(LOCAL_ID).map(|s| s.url.clone()))?
        .ok_or("no local server")?;
    let key = local_key(app, &base).await.map_err(|e| match e {
        ServerError::BadKey => "Hylo's local service is running but refused this app's key.".to_string(),
        e => format!("Hylo's local service stopped answering: {}", e.message()),
    })?;
    Ok(Local { base, key })
}

/// Starts the local server if it isn't up, and waits until it answers.
#[tauri::command]
pub async fn onboarding_ready(app: AppHandle) -> Result<(), String> {
    ready(&app).await.map(|_| ())
}

#[derive(Deserialize)]
struct AgentsResponse {
    agents: Vec<AgentInfo>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AgentInfo {
    id: String,
    name: String,
    available: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    version: String,
}

#[tauri::command]
pub async fn onboarding_agents(app: AppHandle) -> Result<Vec<AgentInfo>, String> {
    let local = ready(&app).await?;
    // Probing runs every agent CLI once, model lists included.
    let http = client(Some(Duration::from_secs(45)))?;
    let body: AgentsResponse = fetch_json(local.req(&http, Method::GET, "/api/agents?force=true"))
        .await
        .map_err(|e| format!("Couldn't look for AI agents: {e}"))?;
    Ok(body.agents)
}

#[derive(Debug, Default, Serialize, PartialEq)]
pub struct Hello {
    ok: bool,
    text: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    error: String,
}

/// Folds one SSE frame of a run into `hello`; true once the run has ended.
fn apply_frame(frame: &str, hello: &mut Hello, on_delta: &mut impl FnMut(&str)) -> bool {
    let mut event = "";
    let mut data = String::new();
    for line in frame.lines() {
        if let Some(v) = line.strip_prefix("event:") {
            event = v.trim();
        } else if let Some(v) = line.strip_prefix("data:") {
            data.push_str(v.trim_start());
        }
    }
    let Ok(v) = serde_json::from_str::<Value>(&data) else {
        return false;
    };
    match event {
        "agent" if v["type"] == "text_delta" => {
            if let Some(d) = v["delta"].as_str() {
                hello.text.push_str(d);
                on_delta(d);
            }
        }
        "error" => {
            if let Some(m) = v["message"].as_str() {
                hello.error = m.to_string();
            }
        }
        "end" => {
            hello.ok = v["status"] == "succeeded" && !hello.text.trim().is_empty();
            return true;
        }
        _ => {}
    }
    false
}

#[derive(Debug, Default, Deserialize)]
pub struct HelloContext {
    #[serde(default)]
    lang: String,
    #[serde(default)]
    now: String,
}

/// The hello run still going, if any: a new hello (another agent picked)
/// cancels it rather than leave two agent CLIs running.
static HELLO_RUN: Mutex<Option<String>> = Mutex::new(None);

/// Asks the agent to say hello and streams its words to the page, which is
/// both the proof that it works and the first thing it ever says. `token`
/// tags the streamed events, so the page only takes the latest hello's.
#[tauri::command]
pub async fn onboarding_hello(
    app: AppHandle,
    agent_id: String,
    token: u64,
    context: Option<HelloContext>,
) -> Result<Hello, String> {
    let local = ready(&app).await?;
    let http = client(Some(Duration::from_secs(10)))?;
    let previous = HELLO_RUN.lock().ok().and_then(|mut r| r.take());
    if let Some(run) = previous {
        let _ = local.req(&http, Method::POST, &format!("/api/runs/{run}/cancel")).send().await;
    }
    let ctx = context.unwrap_or_default();
    let started: Value = fetch_json(
        local
            .req(&http, Method::POST, "/api/setup/hello")
            .json(&json!({ "agentId": agent_id, "lang": ctx.lang, "now": ctx.now })),
    )
    .await?;
    let run = started["runId"].as_str().ok_or("no run id")?.to_string();
    if let Ok(mut slot) = HELLO_RUN.lock() {
        *slot = Some(run.clone());
    }

    let stream = client(None)?;
    let mut hello = Hello::default();
    let read = async {
        let mut resp = local
            .req(&stream, Method::GET, &format!("/api/runs/{run}/events"))
            .send()
            .await
            .map_err(|e| e.to_string())?;
        let mut buf = String::new();
        let mut emit = |d: &str| {
            let _ = app.emit(HELLO_EVENT, json!({ "token": token, "delta": d }));
        };
        while let Some(chunk) = resp.chunk().await.map_err(|e| e.to_string())? {
            buf.push_str(&String::from_utf8_lossy(&chunk));
            while let Some(end) = buf.find("\n\n") {
                let frame: String = buf.drain(..end + 2).collect();
                if apply_frame(&frame, &mut hello, &mut emit) {
                    return Ok::<(), String>(());
                }
            }
        }
        Ok(())
    };
    let outcome = tokio::time::timeout(HELLO_WAIT, read).await;
    if let Ok(mut slot) = HELLO_RUN.lock() {
        if slot.as_deref() == Some(run.as_str()) {
            *slot = None;
        }
    }
    match outcome {
        Ok(r) => r?,
        Err(_) => {
            let _ = local
                .req(&http, Method::POST, &format!("/api/runs/{run}/cancel"))
                .send()
                .await;
            hello.error = "It didn't answer in time.".into();
        }
    }
    Ok(hello)
}

/// Gives the chat a bot to talk to; without one the sidebar is empty. Its
/// chat opens with `greeting`, the hello the user just read.
/// Returns the new bot's id, or None when the vault already had bots.
/// The server owns what the bot is, so the web's own setup makes the same one.
#[tauri::command]
pub async fn onboarding_create_bot(
    app: AppHandle,
    agent_id: String,
    greeting: Option<String>,
) -> Result<Option<String>, String> {
    let local = ready(&app).await?;
    let http = client(Some(Duration::from_secs(10)))?;
    let created: Value = fetch_json(
        local
            .req(&http, Method::POST, "/api/setup/bot")
            .json(&json!({ "agentId": agent_id, "greeting": greeting.unwrap_or_default() })),
    )
    .await?;
    Ok(created["mateId"].as_str().filter(|id| !id.is_empty()).map(str::to_string))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn run(frames: &[&str]) -> (Hello, String, bool) {
        let mut hello = Hello::default();
        let mut streamed = String::new();
        let mut ended = false;
        for f in frames {
            ended = apply_frame(f, &mut hello, &mut |d| streamed.push_str(d));
        }
        (hello, streamed, ended)
    }

    #[test]
    fn streams_text_until_end() {
        let (hello, streamed, ended) = run(&[
            "id: 1\nevent: start\ndata: {}\n\n",
            "id: 2\nevent: agent\ndata: {\"type\":\"text_delta\",\"delta\":\"Hi \"}\n\n",
            "id: 3\nevent: agent\ndata: {\"type\":\"tool_use\"}\n\n",
            "id: 4\nevent: agent\ndata: {\"type\":\"text_delta\",\"delta\":\"there\"}\n\n",
            "id: 5\nevent: end\ndata: {\"status\":\"succeeded\"}\n\n",
        ]);
        assert!(ended && hello.ok);
        assert_eq!(hello.text, "Hi there");
        assert_eq!(streamed, "Hi there");
    }

    #[test]
    fn failed_run_keeps_its_error() {
        let (hello, _, ended) = run(&[
            "event: error\ndata: {\"message\":\"not logged in\"}\n\n",
            "event: end\ndata: {\"status\":\"failed\"}\n\n",
        ]);
        assert!(ended && !hello.ok);
        assert_eq!(hello.error, "not logged in");
    }

    #[test]
    fn success_without_words_is_not_a_hello() {
        let (hello, _, _) = run(&["event: end\ndata: {\"status\":\"succeeded\"}\n\n"]);
        assert!(!hello.ok);
    }
}
