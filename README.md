# Nam's Morning

A calm daily news page: gold, Vietnamese real estate, child care, engineering, AI practice and podcasts. Short summaries with links to the originals, plus a listen/read later list.

**Read it:** https://namvtran-bink.github.io/nam-news/

## How it works

- Every morning at 05:48 (Asia/Ho_Chi_Minh), a scheduled Claude routine follows [`prompts/morning-run.md`](prompts/morning-run.md). It researches the topics in [`config/topics.json`](config/topics.json) and commits that day's news as `data/days/<date>.json`.
- GitHub Pages serves this repo's `main` branch as-is. [`index.html`](index.html) loads `config/topics.json`, `data/index.json` and the day files and shows them. There is no build step.
- Every day is kept. The page opens on the latest day, and "Earlier days" goes back through history.
- The later list is saved in your browser, so the phone and the laptop each keep their own.

## Change things

- **Topics:** edit `config/topics.json`. Order, names, rhythm and what counts as worth reading all live there. The next morning's run uses it.
- **Data format:** see [`data/README.md`](data/README.md).

## Cost

$0. GitHub Pages on a public repo is free, and the routine runs on Nam's Claude plan.
