"use client";

import type { Section } from "@/lib/types";
import StoryRow from "./StoryRow";
import TryingBanner from "./TryingBanner";

type Props = {
  section: Section;
  changing: boolean;
  onChange: (section: Section) => void;
  onFollow: (topicId: number) => Promise<void>;
  onStop: (section: Section) => Promise<void>;
};

export default function TopicSection({ section, changing, onChange, onFollow, onStop }: Props) {
  const headingId = `topic-${section.topicId}`;
  return (
    <section className="topic" aria-labelledby={headingId}>
      <div className="topic-head">
        <h2 id={headingId} className="topic-name">
          {section.name}
        </h2>
        <div className="topic-links">
          <button type="button" className="link" onClick={() => onChange(section)}>
            Change
          </button>
          {!section.trial && (
            <button type="button" className="link" onClick={() => onStop(section)}>
              Stop
            </button>
          )}
        </div>
      </div>
      {section.trial && (
        <TryingBanner topicId={section.topicId} understood={section.understood} onFollow={() => onFollow(section.topicId)} />
      )}
      {changing ? (
        <Skeleton />
      ) : section.items.length > 0 ? (
        section.items.map((story) => <StoryRow key={story.url} story={story} />)
      ) : (
        <p className="empty">Nothing worth your time here today.</p>
      )}
    </section>
  );
}

export function Skeleton() {
  return (
    <div className="skeleton" aria-busy="true" aria-label="Looking for news">
      {[0, 1, 2].map((i) => (
        <div key={i} className="skeleton-row">
          <span className="bar wide" />
          <span className="bar narrow" />
        </div>
      ))}
    </div>
  );
}
