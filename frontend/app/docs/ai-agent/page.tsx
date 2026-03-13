import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "@/features/docs/components/DocsContentPage";

const page = docsPages.aiAgent;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsAiAgentPage() {
  return <DocsContentPage page={page} />;
}
