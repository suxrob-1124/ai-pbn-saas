package legacy

import "testing"

func TestDecodePrimaryHTTPSLinkSingleExternal(t *testing.T) {
	html := `<!doctype html><html><head><title>x</title></head><body>
<a href="/">Home</a>
<p>Text <a href="https://target.example/page">Anchor Text</a></p>
</body></html>`
	link, err := DecodePrimaryHTTPSLink(html, "example.com")
	if err != nil {
		t.Fatalf("DecodePrimaryHTTPSLink failed: %v", err)
	}
	if link == nil {
		t.Fatalf("expected decoded link")
	}
	if link.TargetURL != "https://target.example/page" {
		t.Fatalf("unexpected target url: %s", link.TargetURL)
	}
	if link.AnchorText != "Anchor Text" {
		t.Fatalf("unexpected anchor: %s", link.AnchorText)
	}
	if link.FoundLine < 1 {
		t.Fatalf("unexpected line: %d", link.FoundLine)
	}
}

func TestDecodePrimaryHTTPSLinkMultipleExternalChoosesFirst(t *testing.T) {
	html := `<!doctype html><html><body>
<p><a href="https://first.example">First</a></p>
<p><a href="https://second.example">Second</a></p>
</body></html>`
	link, err := DecodePrimaryHTTPSLink(html, "example.com")
	if err != nil {
		t.Fatalf("DecodePrimaryHTTPSLink failed: %v", err)
	}
	if link == nil {
		t.Fatalf("expected decoded link")
	}
	if link.TargetURL != "https://first.example" {
		t.Fatalf("unexpected target url: %s", link.TargetURL)
	}
}

func TestDecodePrimaryHTTPSLinkNoExternal(t *testing.T) {
	html := `<!doctype html><html><body>
<a href="/">Home</a>
<a href="https://example.com/about">About</a>
</body></html>`
	link, err := DecodePrimaryHTTPSLink(html, "example.com")
	if err != nil {
		t.Fatalf("DecodePrimaryHTTPSLink failed: %v", err)
	}
	if link != nil {
		t.Fatalf("expected nil link when only same-domain links")
	}
}

func TestDecodePrimaryHTTPSLinkBrokenHTML(t *testing.T) {
	html := `<html><body><a href="https://target.example">Broken`
	link, err := DecodePrimaryHTTPSLink(html, "example.com")
	if err != nil {
		t.Fatalf("DecodePrimaryHTTPSLink failed: %v", err)
	}
	if link == nil {
		t.Fatalf("expected decoded link for broken html")
	}
}
