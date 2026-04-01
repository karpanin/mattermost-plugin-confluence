package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDescendantCommentsFromRootResults(t *testing.T) {
	response := &DescendantCommentSearchResponse{
		Results: []CommentResponse{{ID: "123"}},
	}

	comments := getDescendantComments(response)

	require.Len(t, comments, 1)
	require.Equal(t, "123", comments[0].ID)
}

func TestGetDescendantCommentsFromCommentEnvelope(t *testing.T) {
	response := &DescendantCommentSearchResponse{
		Comment: CommentSearchResponse{
			Results: []CommentResponse{{ID: "456"}},
		},
	}

	comments := getDescendantComments(response)

	require.Len(t, comments, 1)
	require.Equal(t, "456", comments[0].ID)
}

func TestFormatMattermostPostForConfluenceMarkdown(t *testing.T) {
	body := formatMattermostPostForConfluence("# Title\n\nThis is **bold** and *italic* with a [link](https://example.com).", "https://mattermost.example.com/_redirect/pl/abc")

	require.Contains(t, body, "<h1>Title</h1>")
	require.Contains(t, body, "<strong>bold</strong>")
	require.Contains(t, body, "<em>italic</em>")
	require.Contains(t, body, "<a href=\"https://example.com\">link</a>")
	require.Contains(t, body, "View original message in Mattermost")
}

func TestFormatMattermostPostForConfluenceCodeBlock(t *testing.T) {
	body := formatMattermostPostForConfluence("```go\nfmt.Println(\"hi\")\n```", "https://mattermost.example.com/_redirect/pl/abc")

	require.Contains(t, body, "<ac:structured-macro ac:name=\"code\">")
	require.Contains(t, body, "<ac:parameter ac:name=\"language\">go</ac:parameter>")
	require.Contains(t, body, "fmt.Println(\"hi\")")
}
