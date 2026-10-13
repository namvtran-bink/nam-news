# Data

Plain JSON, one file per day. The page reads these files directly; nothing else is needed to serve them.

## `data/index.json`

```json
{ "days": ["2026-10-13", "2026-10-12"] }
```

Every date that has a file in `data/days/`, newest first.

## `data/days/<YYYY-MM-DD>.json`

```jsonc
{
  "date": "2026-10-13",
  "gold_prices": {
    "as_of": "Domestic prices as of 05:00, world price this morning, both from Báo Đà Nẵng.",
    "source_url": "https://…",
    "rows": [
      { "type": "sjc_bar",        "label": "SJC miếng",      "currency": "VND", "unit": "million VND per lượng", "buy": 141.3, "sell": 144.3, "change": -0.5 },
      { "type": "btmc_ring_9999", "label": "Nhẫn 9999 BTMC", "currency": "VND", "unit": "million VND per lượng", "buy": 139.4, "sell": 143.4, "change": -0.2 },
      { "type": "world_spot",     "label": "World spot",     "currency": "USD", "unit": "USD per troy ounce", "price": 4172.36, "change_pct": -0.5 }
    ]
  },
  "notes": { "childcare": "Nothing worth your time today." },
  "items": [
    {
      "id": "3f1c…",              // first 12 hex chars of sha1(url)
      "topic": "gold",            // a key from config/topics.json
      "title": "…",
      "url": "https://…",         // the original article or episode
      "source": "Báo Đà Nẵng",
      "published": "2026-10-13",  // ISO date, or null when the source gives no exact date
      "lang": "vi",               // language of the source and of the summary: "vi" or "en"
      "kind": "Read",             // "Read" or "Listen"
      "summary": "…",             // 1–2 sentences, or null when the source is too thin
      "duration": "1 h 58 min"    // podcasts only, optional
    }
  ]
}
```

`change` is the change since the previous day in the same unit as `buy`/`sell`. `notes` holds a line for a topic that has nothing today, keyed by topic.

The rows map straight onto two tables if a database is added later: `items` (one row per item, with `date` from the file) and `gold_prices` (one row per price per day).
