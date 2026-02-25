import { AdminOnlyDocs } from "../../../components/AdminOnlyDocs";
import { docsPages } from "../../../docs-content/registry";
import { DocsContentPage } from "../../../features/docs/components/DocsContentPage";

const page = docsPages.indexingApi;

export const metadata = {
  title: page.title,
  description: page.description,
};

export default function DocsIndexingApiPage() {
  return (
    <AdminOnlyDocs title="API проверок индексации для администраторов">
      <DocsContentPage page={page} />
    </AdminOnlyDocs>
  );
}
