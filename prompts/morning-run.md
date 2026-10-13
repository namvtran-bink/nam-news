# Morning run

The scheduled Claude routine "Morning news" runs this every day at 05:48 Asia/Ho_Chi_Minh. It changes nothing except the data files below.

1. Get the latest `main` of this repo.
2. Today's date `D` is the current date in Asia/Ho_Chi_Minh (YYYY-MM-DD).
3. Read `config/topics.json` for the topics, their order, `window_days`, `max_items` and `brief`.
4. Read the day files from the last 14 days in `data/days/` and collect their URLs. Never repeat a URL that was shown on an earlier day.
5. For each topic, search the web, then open every article you plan to include and write the summary only from what that page actually says.
   - Never invent numbers, dates, names or durations. If a page won't open, leave it out.
   - Check each article's real publication date and year in the article itself. Some Vietnamese sites put today's date and today's price table around years-old articles.
   - Keep only items published within the topic's `window_days`, at most `max_items` per topic, and about 6–10 items in total.
   - Fewer, better items. Drop clickbait, opinion threads, press-release rewrites, duplicates of the same story and listicles. An empty topic is fine: add a short line for it to `notes`.
   - The summary has 1–2 sentences and at most about 50 words. It is neutral, in your own words, and **in the source's language** (Vietnamese for Vietnamese sources, English for English ones). Use `null` if the source is too thin to summarize honestly.
6. Gold: always fill `gold_prices` with this morning's SJC bar and BTMC 9999 ring buy/sell prices (million VND per lượng) from a Vietnamese source, their change since yesterday, and the world spot price in USD per troy ounce with its percent change. Then add 1–2 gold items, one on the domestic move and one on the global move, each saying why prices moved. Leave out price predictions.
7. Write `data/days/D.json` following `data/README.md`. Add `D` to the front of `data/index.json`. If `D` already exists, replace that file.
8. Check that both files parse as JSON, then commit them to `main` with the message `News for D` and push.

Never edit other days' files, the page, or the config during a run.
