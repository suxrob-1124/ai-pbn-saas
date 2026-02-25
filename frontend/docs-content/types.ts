export type DocsListType = "bullet" | "numbered";

export type DocsSection = {
  title?: string;
  paragraphs?: string[];
  listType?: DocsListType;
  listItems?: string[];
};

export type DocsPageContent = {
  title: string;
  description: string;
  intro: string;
  sections: DocsSection[];
};
