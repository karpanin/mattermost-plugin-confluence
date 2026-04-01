package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertConfluenceHTMLToMarkdown(t *testing.T) {
	html := `<h1>Title</h1><p>This is <strong>bold</strong>, <em>italic</em> and <a href="https://example.com">link</a>.</p>`

	markdown := ConvertConfluenceHTMLToMarkdown(html)

	require.Contains(t, markdown, "# Title")
	require.Contains(t, markdown, "**bold**")
	require.Contains(t, markdown, "_italic_")
	require.Contains(t, markdown, "[link](https://example.com)")
}
