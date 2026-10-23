import ReactMarkdown, { uriTransformer } from "react-markdown";

// Bot/PR comments arrive as GitHub-flavored markdown sprinkled with HTML
// metadata. react-markdown escapes raw HTML (safe), which would render
// comment markers and <sup> wrappers as visible noise — strip them first.
function preprocess(md: string): string {
  return md
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/<\/?(sup|sub|details|summary|br|p|div|b|i|em|strong)\s*\/?>/gi, "")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

export function MarkdownBody({
  text,
  onLinkClick,
}: {
  text: string;
  // When set, anchors don't navigate; the handler gets (href, link text) and
  // decides what to do (open an app tab, the system browser, …).
  onLinkClick?: (href: string, label: string) => void;
}) {
  return (
    <div className="md-body">
      <ReactMarkdown
        // The default sanitizer strips the app's context:// scheme. Allowing it
        // is safe only because with onLinkClick anchors never actually navigate.
        transformLinkUri={
          onLinkClick ? (uri) => (uri.startsWith("context://") ? uri : uriTransformer(uri)) : uriTransformer
        }
        components={
          onLinkClick && {
            a: ({ href, children }) => (
              <a
                href={href}
                onClick={(e) => {
                  e.preventDefault();
                  onLinkClick(href ?? "", e.currentTarget.textContent ?? "");
                }}
              >
                {children}
              </a>
            ),
          } || undefined
        }
      >
        {preprocess(text)}
      </ReactMarkdown>
    </div>
  );
}
