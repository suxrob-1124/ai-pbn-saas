export type DocsListType = "bullet" | "numbered";

export type DocsCodeBlock = {
  code: string;
  caption?: string;
};

export type DocsSection = {
  title?: string;
  paragraphs?: string[];
  listType?: DocsListType;
  listItems?: string[];
  codeBlocks?: DocsCodeBlock[];
  adminOnly?: boolean;
};

export type DocsPageContent = {
  title: string;
  description: string;
  intro: string;
  sections: DocsSection[];
};
