package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
)

func TestExtractMentionIdentifiers(t *testing.T) {
	body := `<ac:link><ri:user ri:userkey="user-key-1" /></ac:link><span data-userkey="user-key-2"></span>`
	identifiers := extractMentionIdentifiers(body)

	assert.Contains(t, identifiers, "user-key-1")
	assert.Contains(t, identifiers, "user-key-2")
}

func TestExtractMentionIdentifiersFromConfluenceViewHTML(t *testing.T) {
	body := `<p><a class="confluence-userlink user-mention" data-linked-resource-id="user-key-3" data-username="alice">@Alice</a></p>`
	identifiers := extractMentionIdentifiers(body)

	assert.Contains(t, identifiers, "user-key-3")
	assert.Contains(t, identifiers, "alice")
}

func TestBuildPersonalNotificationMessage(t *testing.T) {
	event := &ConfluenceServerEvent{
		Comment: &CommentResponse{
			Container: CommentContainer{
				Title: "Architecture page",
				Links: Links{Self: "/pages/123"},
			},
			Space: SpaceResponse{
				Key:   "ENG",
				Name:  "Engineering",
				Links: Links{Self: "/spaces/ENG"},
			},
			Links: Links{Self: "/comments/456"},
		},
		Page: &PageResponse{
			Title: "Architecture page",
			Space: SpaceResponse{
				Key:   "ENG",
				Name:  "Engineering",
				Links: Links{Self: "/spaces/ENG"},
			},
			Links: Links{Self: "/pages/123"},
		},
	}

	mentionMessage := buildPersonalNotificationMessage(notificationTypeMention, serializer.CommentCreatedEvent, event, "https://conf.example.com", "Alice")
	assert.Contains(t, mentionMessage, "Alice mentioned you")

	watchingMessage := buildPersonalNotificationMessage(notificationTypeWatching, serializer.PageUpdatedEvent, event, "https://conf.example.com", "Alice")
	assert.Contains(t, watchingMessage, "You are watching this page in Confluence")
}
