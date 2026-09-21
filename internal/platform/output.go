package platform

import "fmt"

// Presentation is independent of the JSON stdout protocol and SSE transport.
func outputType(value string) (string, error) {
	switch value {
	case "", "json", "application/json":
		return "json", nil
	case "text", "text/plain":
		return "text", nil
	case "html", "text/html":
		return "html", nil
	case "csv", "text/csv":
		return "csv", nil
	case "xml", "text/xml", "application/xml":
		return "xml", nil
	case "md", "markdown", "text/markdown":
		return "markdown", nil
	case "stream", "text/event-stream":
		return "stream", nil
	case "image-gallery":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported output_schema.type %q; use json, text, html, markdown, csv, xml, image-gallery or stream", value)
	}
}

func runOutputType(m Manifest) string {
	if m.Execution.Stream {
		return "stream"
	} // Existing SSE tools predate the strict declaration.
	t, err := outputType(m.Output.Type)
	if err != nil {
		return "json"
	} // Unknown legacy declarations never enable HTML.
	return t
}
