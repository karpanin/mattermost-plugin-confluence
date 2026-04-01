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
			Body: Body{
				View: View{
					Value: `<p><a class="confluence-userlink user-mention" data-linked-resource-id="user-key-3">@Alice</a> У меня зависла заявка на доступы.</p>`,
				},
			},
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
	assert.Contains(t, mentionMessage, "> @Alice")
	assert.Contains(t, mentionMessage, "> У меня зависла заявка на доступы.")

	watchingMessage := buildPersonalNotificationMessage(notificationTypeWatching, serializer.PageUpdatedEvent, event, "https://conf.example.com", "Alice")
	assert.Contains(t, watchingMessage, "You are watching this page in Confluence")
}
