export type DomainProjectRole = "admin" | "owner" | "manager" | "editor" | "viewer";

export type GenerationView = {
  status: string;
  progress: number;
};

export type LinkTaskView = {
  status: string;
  action?: string;
};

export type DomainLinkView = {
  link_status?: string;
  link_status_effective?: string;
  link_last_task_id?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
};
