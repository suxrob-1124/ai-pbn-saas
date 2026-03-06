import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.indexing;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsIndexingPage() {
  return <DocsContentPage page={page} />;
}
