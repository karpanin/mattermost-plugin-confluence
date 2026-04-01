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
