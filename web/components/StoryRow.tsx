import type { Story } from "@/lib/types";

export default function StoryRow({ story }: { story: Story }) {
  return (
    <article className="story">
      <h3 className="headline">
        <a href={story.url} target="_blank" rel="noopener noreferrer">
          {story.title}
        </a>
      </h3>
      <p className="byline">
        {story.source} · {story.date}
      </p>
      {story.summary && <p className="summary">{story.summary}</p>}
    </article>
  );
}
