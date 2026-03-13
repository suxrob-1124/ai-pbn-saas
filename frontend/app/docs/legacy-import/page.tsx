import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.legacyImport;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsLegacyImportPage() {
  return <DocsContentPage page={page} />;
}
