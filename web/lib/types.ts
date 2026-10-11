// Mirrors the backend's GET /api/today JSON (see CLAUDE.md section 6).

export type Story = {
  title: string;
  url: string;
  source: string;
  date: string;
  summary: string | null;
};

export type Section = {
  topicId: number;
  name: string;
  trial: boolean;
  understood: string | null;
  items: Story[];
};

export type Page = {
  date: string;
  readMinutes: number;
  count: number;
  sections: Section[];
};
