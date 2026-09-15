// a2a.rs — Rust client for the Quilt a2a-protocol Worker.
//
// a2a (agent-to-agent) is how cells on different machines discover
// each other. The Worker is at https://quilt-a2a-v2.casey-digennaro.workers.dev

use reqwest::Client;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct Cell {
    pub cell_id: String,
    pub role: String,
    pub capabilities: Vec<String>,
}

pub struct A2AClient {
    base: String,
    http: Client,
}

impl A2AClient {
    pub fn new(base: &str) -> Self {
        Self { base: base.to_string(), http: Client::new() }
    }

    pub async fn register(&self, cell_id: &str, caps: Vec<String>) -> Result<(), String> {
        let url = format!("{}/register", self.base);
        let body = serde_json::json!({
            "cell_id": cell_id,
            "role": "rust-cell",
            "capabilities": caps,
        });
        let res = self.http.post(&url).json(&body).send().await
            .map_err(|e| e.to_string())?;
        if !res.status().is_success() {
            return Err(format!("status {}", res.status()));
        }
        Ok(())
    }

    pub async fn find_semantic(&self, query: &str) -> Result<Vec<String>, String> {
        let url = format!("{}/find-semantic?q={}&k=5", self.base, query);
        let res = self.http.get(&url).send().await.map_err(|e| e.to_string())?;
        let body: serde_json::Value = res.json().await.map_err(|e| e.to_string())?;
        Ok(body["matches"].as_array().unwrap_or(&vec![])
            .iter()
            .map(|m| m["id"].as_str().unwrap_or("").to_string())
            .collect())
    }
}
