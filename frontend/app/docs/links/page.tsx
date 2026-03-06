import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.links;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsLinksPage() {
  return <DocsContentPage page={page} />;
}
