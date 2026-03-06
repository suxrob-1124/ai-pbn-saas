import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.domains;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsDomainsPage() {
  return <DocsContentPage page={page} />;
}
