"use client";

import { useEffect, useState } from "react";

// "Just once" is remembered only in this browser; the topic disappears tomorrow anyway.
const onceKey = (id: number) => `quiet-once-${id}`;

type Props = {
  topicId: number;
  understood: string | null;
  onFollow: () => Promise<void>;
};

export default function TryingBanner({ topicId, understood, onFollow }: Props) {
  const [justOnce, setJustOnce] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    try {
      setJustOnce(localStorage.getItem(onceKey(topicId)) === "1");
    } catch {}
  }, [topicId]);

  function chooseOnce() {
    setJustOnce(true);
    try {
      localStorage.setItem(onceKey(topicId), "1");
    } catch {}
  }

  async function follow() {
    setBusy(true);
    try {
      await onFollow();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="trying">
      <p className="trying-label">{justOnce ? "Just for today" : "Trying"}</p>
      {understood && <p className="understood">{understood}</p>}
      {!justOnce && (
        <div className="trying-actions">
          <button type="button" className="primary" onClick={follow} disabled={busy}>
            {busy ? "Following…" : "Follow daily"}
          </button>
          <button type="button" className="secondary" onClick={chooseOnce} disabled={busy}>
            Just once
          </button>
        </div>
      )}
    </div>
  );
}
