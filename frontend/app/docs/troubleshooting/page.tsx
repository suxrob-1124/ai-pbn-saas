import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.troubleshooting;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsTroubleshootingPage() {
  return <DocsContentPage page={page} />;
}
