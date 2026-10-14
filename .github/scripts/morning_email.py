"""Build the morning email comment from the newest day's JSON."""
import json

PAGE = "https://namvtran-bink.github.io/nam-news/"

topics = {t["key"]: t["name"] for t in json.load(open("config/topics.json"))["topics"]}
date = json.load(open("data/index.json"))["days"][0]
day = json.load(open(f"data/days/{date}.json"))

lines = [f"## Today's page is ready · {date}", "", f"**[Open today's page]({PAGE})**", ""]

gold = (day.get("gold_prices") or {}).get("rows") or []
if gold:
    parts = []
    for r in gold:
        if r.get("currency") == "VND":
            parts.append(f"{r['label']} {r['buy']:g} – {r['sell']:g} tr")
        else:
            parts.append(f"{r['label']} ${r['price']:,.0f}")
    lines += ["**Gold:** " + " · ".join(parts), ""]

for key, name in topics.items():
    items = [i for i in day.get("items", []) if i["topic"] == key]
    if not items:
        continue
    lines.append(f"**{name}**")
    for i in items:
        tag = " 🎧" if i.get("kind") == "Listen" else ""
        lines.append(f"- [{i['title']}]({i['url']}){tag} · {i['source']}")
    lines.append("")

with open("comment.md", "w") as f:
    f.write("\n".join(lines))
print("\n".join(lines))
