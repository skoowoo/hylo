// The agent's first words during onboarding. It gets a glimpse of the vault
// (the server's /api/vault/overview) so it can say something only an AI that
// sees these notes could say.
use std::time::{SystemTime, UNIX_EPOCH};

use serde::Deserialize;

const NOTES_ANGLES: &[&str] = &[
    "Notice a theme running through their note titles and name it, like a small revelation.",
    "Pick one note title and link it to another they may not have connected yet.",
    "Pick one note title and ask one curious, specific question about it they'd want to answer.",
    "Offer one concrete thing you could do with these exact notes right now, naming them.",
    "Make a light, perceptive guess at what they've been working on lately, from what changed most recently.",
    "Guess one thing about them as a person, from what they choose to write about, and say it lightly.",
];
const EMPTY_ANGLES: &[&str] = &[
    "Suggest one tiny, specific first note that fits their time of day.",
    "Make a playful, perceptive remark about a blank page, then invite one specific first thought.",
    "Ask one question whose answer would make a great first note.",
    "Tell them, concretely, what you'll do once they've written a few notes, like learning what they care about and finding news that fits.",
];

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Context {
    #[serde(default)]
    lang: String,
    #[serde(default)]
    now: String,
}

#[derive(Debug, Deserialize)]
pub struct Glimpse {
    #[serde(default)]
    name: String,
    #[serde(default)]
    notes: usize,
    #[serde(default)]
    folders: Vec<Named>,
    #[serde(default)]
    recent: Vec<Titled>,
}

#[derive(Debug, Deserialize)]
struct Named {
    name: String,
}

#[derive(Debug, Deserialize)]
struct Titled {
    title: String,
}

fn quoted<'a>(items: impl Iterator<Item = &'a str>) -> String {
    items.map(|s| format!("\"{s}\"")).collect::<Vec<_>>().join(", ")
}

/// Same vault, different words each time: the angle rotates.
pub fn prompt(g: &Glimpse, ctx: &Context, seed: usize) -> String {
    let mut known = Vec::new();
    if !ctx.now.is_empty() {
        known.push(format!("- Their local time: {}.", ctx.now));
    }
    let angle = if g.notes == 0 {
        known.push(format!("- Their notes folder \"{}\" is brand new and empty.", g.name));
        EMPTY_ANGLES[seed % EMPTY_ANGLES.len()]
    } else {
        let plural = if g.notes == 1 { "note" } else { "notes" };
        let mut line = format!("- Their notes folder \"{}\" holds {} {plural}", g.name, g.notes);
        if !g.folders.is_empty() {
            line.push_str(&format!(", mostly in {}", quoted(g.folders.iter().map(|f| f.name.as_str()))));
        }
        known.push(line + ".");
        known.push(format!("- Most recently edited: {}.", quoted(g.recent.iter().map(|r| r.title.as_str()))));
        NOTES_ANGLES[seed % NOTES_ANGLES.len()]
    };
    let lang = if ctx.lang.is_empty() { "English" } else { &ctx.lang };
    format!(
        "You are Hylo, a personal AI agent powered by someone's notes: they just write, and from their notes \
you learn what matters to them, then work for them around the clock, on their notes and beyond. \
They just set you up, and this is the first thing you'll ever say to them. \
It appears in a small card on the setup screen, so make it count.

What you know:
{}

Write one or two short sentences: 25 words at most, or 50 characters in Chinese or Japanese. {angle}
Sound like a sharp, warm friend who already noticed something, not like a product. \
Never say you're ready to help, never ask how you can help, no greeting formula, no emoji, no markdown, no quotation marks around your reply. \
Speak as Hylo: never name the AI model, agent or company behind you. Don't use tools or open files; use only what's above.
Reply in the language their note titles are mostly written in; if there are none, use the language of the locale {lang}.",
        known.join("\n")
    )
}

pub fn seed() -> usize {
    SystemTime::now().duration_since(UNIX_EPOCH).map(|d| d.subsec_nanos() as usize).unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn prompt_matches_the_vault() {
        let ctx = Context { lang: "zh-CN".into(), now: "Monday 21:40".into() };
        let empty: Glimpse = serde_json::from_str(r#"{"name":"Hylo Vault","notes":0,"folders":[],"recent":[]}"#).unwrap();
        let p = prompt(&empty, &ctx, 0);
        assert!(p.contains("brand new and empty") && p.contains(EMPTY_ANGLES[0]) && p.contains("zh-CN"));
        let full: Glimpse = serde_json::from_str(
            r#"{"name":"Notes","notes":12,"folders":[{"name":"Work","count":9}],"recent":[{"path":"/Work/Q3 plan.md","title":"Q3 plan"}]}"#,
        )
        .unwrap();
        let p = prompt(&full, &ctx, 1);
        assert!(p.contains("holds 12 notes, mostly in \"Work\"") && p.contains("\"Q3 plan\"") && p.contains(NOTES_ANGLES[1]));
    }
}
