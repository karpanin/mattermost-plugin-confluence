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
					Value: `<p><a class="confluence-userlink user-mention" data-linked-resource-id="user-key-3">@Alice</a> Ð£ Ð¼ÐµÐ½Ñ Ð·Ð°Ð²Ð¸ÑÐ»Ð° Ð·Ð°ÑÐ²ÐºÐ° Ð½Ð° Ð´Ð¾ÑÑ‚ÑƒÐ¿Ñ‹.</p>`,
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
	assert.Contains(t, mentionMessage, "Ð£ Ð¼ÐµÐ½Ñ Ð·Ð°Ð²Ð¸ÑÐ»Ð° Ð·Ð°ÑÐ²ÐºÐ° Ð½Ð° Ð´Ð¾ÑÑ‚ÑƒÐ¿Ñ‹.")

	watchingMessage := buildPersonalNotificationMessage(notificationTypeWatching, serializer.PageUpdatedEvent, event, "https://conf.example.com", "Alice")
	assert.Contains(t, watchingMessage, "You are watching this page in Confluence")
}

func TestBuildPersonalNotificationMessageForPageMention(t *testing.T) {
	event := &ConfluenceServerEvent{
		Page: &PageResponse{
			Title: "Architecture page",
			Space: SpaceResponse{
				Key:   "ENG",
				Name:  "Engineering",
				Links: Links{Self: "/spaces/ENG"},
			},
			Body: Body{
				View: View{
					Value: `<p><a class="confluence-userlink user-mention" data-linked-resource-id="user-key-3">@Alice</a> Please review this section.</p>`,
				},
			},
			Links: Links{Self: "/pages/123"},
		},
	}

	createdMessage := buildPersonalNotificationMessage(notificationTypeMention, serializer.PageCreatedEvent, event, "https://conf.example.com", "Alice")
	assert.Contains(t, createdMessage, "Alice mentioned you on")
	assert.Contains(t, createdMessage, "[Architecture page](https://conf.example.com/pages/123)")
	assert.NotContains(t, createdMessage, ">")

	updatedMessage := buildPersonalNotificationMessage(notificationTypeMention, serializer.PageUpdatedEvent, event, "https://conf.example.com", "Alice")
	assert.Contains(t, updatedMessage, "Alice mentioned you on")
}
