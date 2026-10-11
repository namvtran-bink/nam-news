"use client";

import type { FormEvent, RefObject } from "react";

type Props = {
  value: string;
  onValueChange: (value: string) => void;
  onSubmit: (text: string) => void;
  onCancelChange: () => void;
  changingName: string | null; // set when the bar is editing an existing topic
  busy: boolean;
  error: string | null;
  inputRef: RefObject<HTMLInputElement | null>;
  docked: boolean;
};

export default function AskBar(props: Props) {
  const { value, onValueChange, onSubmit, onCancelChange, changingName, busy, error, inputRef, docked } = props;

  function submit(e: FormEvent) {
    e.preventDefault();
    const text = value.trim();
    if (text && !busy) onSubmit(text);
  }

  const label = changingName ? `Change ${changingName}` : "Follow something new";
  return (
    <form className={docked ? "askbar docked" : "askbar"} onSubmit={submit}>
      <div className="askbar-inner">
        {changingName && (
          <p className="askbar-mode">
            Changing <strong>{changingName}</strong>. Say what to adjust, like &ldquo;less company news&rdquo;.{" "}
            <button type="button" className="link" onClick={onCancelChange}>
              Cancel
            </button>
          </p>
        )}
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        <div className="askbar-row">
          <label htmlFor="ask" className="visually-hidden">
            {label}
          </label>
          <input
            id="ask"
            ref={inputRef}
            value={value}
            onChange={(e) => onValueChange(e.target.value)}
            placeholder="Follow something new…"
            autoComplete="off"
            maxLength={300}
            disabled={busy}
          />
          <button type="submit" className="primary" disabled={busy || !value.trim()}>
            {busy ? "Looking…" : changingName ? "Update" : "Follow"}
          </button>
        </div>
      </div>
    </form>
  );
}
