export type EditorMode = "content" | "code";

export type SEOData = {
  title: string;
  description: string;
  ogTitle: string;
  ogDescription: string;
};

export type ContentPage = {
  path: string;
  displayName: string;
  isIndex: boolean;
};

export type CreatePageTemplate = {
  id: string;
  label: string;
  icon: string;
  prompt: string;
};
