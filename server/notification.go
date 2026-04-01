package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/thoas/go-funk"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

var eventActions = map[string]string{
	serializer.PageCreatedEvent:  "published",
	serializer.PageUpdatedEvent:  "updated",
	serializer.PageTrashedEvent:  "trashed",
	serializer.PageRestoredEvent: "restored",
	serializer.PageRemovedEvent:  "removed",
}

const (
	notificationTypeMention  = "mention"
	notificationTypeWatching = "watching"
)

var mentionIdentifierPatterns = []*regexp.Regexp{
	regexp.MustCompile(`data-linked-resource-id="([^"]+)"`),
	regexp.MustCompile(`data-userkey="([^"]+)"`),
	regexp.MustCompile(`ri:userkey="([^"]+)"`),
	regexp.MustCompile(`data-username="([^"]+)"`),
}

type notification struct {
	*Plugin
}

func (p *Plugin) getNotification() *notification {
	return &notification{
		p,
	}
}

func (n *notification) SendConfluenceNotifications(event serializer.ConfluenceEventV2, eventType, botUserID string, eventTriggerer string, eventTriggererKey string) {
	url := event.GetURL()
	if url == "" {
		return
	}

	spaceKey, pageID := n.extractSpaceKeyAndPageID(event, eventType)
	post := event.GetNotificationPost(eventType, url, botUserID, eventTriggerer)
	if post != nil && spaceKey != "" && pageID != "" {
		subscriptionChannelIDs := n.getNotificationChannelIDs(url, spaceKey, pageID, eventType)
		for _, channelID := range subscriptionChannelIDs {
			post.ChannelId = channelID
			if _, err := n.API.CreatePost(post); err != nil {
				n.API.LogError("Unable to create Post in Mattermost", "Error", err.Error())
			}
		}
	}

	n.sendPersonalNotifications(event, eventType, url, pageID, botUserID, eventTriggerer, eventTriggererKey)
}

func (n *notification) SendGenericWHNotification(event *serializer.ConfluenceServerWebhookPayload, botUserID, url string) {
	eventType := event.Event

	action, exists := eventActions[eventType]
	if !exists {
		n.client.Log.Info("Unsupported Confluence action. Generic notification will not be sent", "event type", eventType)
		return
	}

	post := &model.Post{
		UserId:  botUserID,
		Message: fmt.Sprintf("Someone %s a page on Confluence with the id %d", action, event.Page.ID),
	}

	urlPageIDSubscriptions, err := service.GetSubscriptionsByURLPageID(url, strconv.FormatInt(event.Page.ID, 10))
	if err != nil {
		n.API.LogError("Unable to get subscribed channels for pageID.", event.Page.ID, "Error", err.Error())
		return
	}

	subscriptionChannelIDs := GetURLSubscriptionChannelIDs(urlPageIDSubscriptions, eventType)
	for _, channelID := range subscriptionChannelIDs {
		post.ChannelId = channelID
		if _, err := n.API.CreatePost(post); err != nil {
			n.API.LogError("Unable to create Post in Mattermost", "Error", err.Error())
		}
	}
}

func (n *notification) extractSpaceKeyAndPageID(event serializer.ConfluenceEventV2, eventType string) (string, string) {
	var spaceKey, pageID string

	switch {
	case strings.Contains(eventType, Comment):
		if e, ok := event.(*ConfluenceServerEvent); ok {
			if e.Comment == nil {
				return "", ""
			}

			spaceKey = e.GetCommentSpaceKey()
			pageID = e.GetCommentContainerID()
		}
	case strings.Contains(eventType, Page):
		if e, ok := event.(*ConfluenceServerEvent); ok {
			if e.Page == nil {
				return "", ""
			}

			spaceKey = e.GetPageSpaceKey()
			pageID = event.GetPageID()
		}
	case strings.Contains(eventType, Space):
		spaceKey = event.GetSpaceKey()
		if spaceKey != "" {
			pageID = ""
		}
	}

	return spaceKey, pageID
}

func (n *notification) getNotificationChannelIDs(url, spaceKey, pageID, eventType string) []string {
	urlSpaceKeySubscriptions, err := service.GetSubscriptionsByURLSpaceKey(url, spaceKey)
	if err != nil {
		n.API.LogError("Unable to get subscribed channels for spaceKey", "SpaceKey", spaceKey, "Error", err.Error())
		return nil
	}
	urlPageIDSubscriptions, err := service.GetSubscriptionsByURLPageID(url, pageID)
	if err != nil {
		n.API.LogError("Unable to get subscribed channels for page", "PageID", pageID, "Error", err.Error())
		return nil
	}

	urlPageIDSubscriptionChannelIDs := GetURLSubscriptionChannelIDs(urlPageIDSubscriptions, eventType)
	urlSpaceKeySubscriptionChannelIDs := GetURLSubscriptionChannelIDs(urlSpaceKeySubscriptions, eventType)

	return util.Deduplicate(append(urlSpaceKeySubscriptionChannelIDs, urlPageIDSubscriptionChannelIDs...))
}

func GetURLSubscriptionChannelIDs(urlSubscriptions serializer.StringArrayMap, eventType string) []string {
	var urlSubscriptionChannelIDs []string

	for channelID, events := range urlSubscriptions {
		if funk.Contains(events, eventType) {
			urlSubscriptionChannelIDs = append(urlSubscriptionChannelIDs, channelID)
		}
	}

	return urlSubscriptionChannelIDs
}

func (n *notification) sendPersonalNotifications(event serializer.ConfluenceEventV2, eventType, baseURL, pageID, botUserID, eventTriggerer, eventTriggererKey string) {
	recipients := map[string]string{}
	if strings.Contains(eventType, Comment) {
		for _, userID := range n.resolveMentionedUserIDs(baseURL, event, eventTriggererKey) {
			recipients[userID] = notificationTypeMention
		}
	}

	if eventType == serializer.PageUpdatedEvent {
		for _, userID := range n.resolveWatcherUserIDs(pageID, baseURL, eventTriggererKey) {
			if _, exists := recipients[userID]; !exists {
				recipients[userID] = notificationTypeWatching
			}
		}
	}

	for userID, reason := range recipients {
		if userID == "" {
			continue
		}

		if err := n.sendPersonalNotificationToUser(userID, reason, eventType, event, baseURL, botUserID, eventTriggerer); err != nil {
			n.API.LogError("Unable to send personal Confluence notification", "UserID", userID, "Error", err.Error())
		}
	}
}

func (n *notification) resolveMentionedUserIDs(instanceID string, event serializer.ConfluenceEventV2, eventTriggererKey string) []string {
	serverEvent, ok := event.(*ConfluenceServerEvent)
	if !ok || serverEvent.Comment == nil {
		return nil
	}

	mentionedIDs := map[string]struct{}{}
	for _, identifier := range extractMentionIdentifiers(getCommentMentionSource(serverEvent.Comment)) {
		if identifier == "" || identifier == eventTriggererKey {
			continue
		}

		mmUserID, err := store.GetMattermostUserIDFromConfluenceID(instanceID, identifier)
		if err != nil || mmUserID == nil || *mmUserID == "" {
			continue
		}
		mentionedIDs[*mmUserID] = struct{}{}
	}

	return mapKeys(mentionedIDs)
}

func (n *notification) resolveWatcherUserIDs(pageID, instanceID, eventTriggererKey string) []string {
	if pageID == "" {
		return nil
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.AdminAPIToken == "" {
		return nil
	}

	watchers, err := n.GetContentWatchersWithAPIToken(pageID, pluginConfig)
	if err != nil {
		n.API.LogError("Unable to get Confluence content watchers", "PageID", pageID, "Error", err.Error())
		return nil
	}

	mmUserIDs := map[string]struct{}{}
	for _, watcher := range watchers {
		identifier := watcher.UserKey
		if identifier == "" {
			identifier = watcher.Username
		}
		if identifier == "" || identifier == eventTriggererKey {
			continue
		}

		mmUserID, lookupErr := store.GetMattermostUserIDFromConfluenceID(instanceID, identifier)
		if lookupErr != nil || mmUserID == nil || *mmUserID == "" {
			continue
		}

		mmUserIDs[*mmUserID] = struct{}{}
	}

	return mapKeys(mmUserIDs)
}

func (n *notification) sendPersonalNotificationToUser(userID, reason, eventType string, event serializer.ConfluenceEventV2, baseURL, botUserID, eventTriggerer string) error {
	connection, err := store.LoadConnection(baseURL, userID)
	if err != nil {
		return err
	}
	if connection == nil {
		return errors.New("connection is nil")
	}

	if !connection.ShouldReceiveNotification(reason) {
		return nil
	}

	message := buildPersonalNotificationMessage(reason, eventType, event, baseURL, eventTriggerer)
	if message == "" {
		return nil
	}

	channel, appErr := n.client.Channel.GetDirect(userID, botUserID)
	if appErr != nil {
		return errors.New(appErr.Error())
	}
	if channel == nil {
		return errors.New("direct channel is nil")
	}

	_, appErr = n.API.CreatePost(&model.Post{
		UserId:    botUserID,
		ChannelId: channel.Id,
		Message:   message,
	})
	if appErr != nil {
		return errors.New(appErr.Error())
	}

	return nil
}

func buildPersonalNotificationMessage(reason, eventType string, event serializer.ConfluenceEventV2, baseURL, eventTriggerer string) string {
	serverEvent, ok := event.(*ConfluenceServerEvent)
	if !ok {
		return ""
	}

	switch reason {
	case notificationTypeMention:
		if serverEvent.Comment == nil {
			return ""
		}
		pageName := serverEvent.GetPageDisplayNameForCommentEvents(baseURL)
		spaceName := serverEvent.GetSpaceDisplayNameForCommentEvents(baseURL)
		commentURL := joinURL(baseURL, serverEvent.Comment.Links.Self)
		commentExcerpt := getCommentExcerpt(serverEvent.Comment)
		if commentExcerpt != "" {
			return fmt.Sprintf("%s mentioned you in a [comment](%s) on %s in %s.\n> %s", eventTriggerer, commentURL, pageName, spaceName, strings.ReplaceAll(commentExcerpt, "\n", "\n> "))
		}
		return fmt.Sprintf("%s mentioned you in a [comment](%s) on %s in %s.", eventTriggerer, commentURL, pageName, spaceName)
	case notificationTypeWatching:
		if eventType != serializer.PageUpdatedEvent || serverEvent.Page == nil {
			return ""
		}
		pageName := serverEvent.GetPageDisplayNameForPageEvents(baseURL)
		spaceName := serverEvent.GetSpaceDisplayNameForPageEvents(baseURL)
		return fmt.Sprintf("%s updated %s in %s.\n\n*You are watching this page in Confluence.*", eventTriggerer, pageName, spaceName)
	default:
		return ""
	}
}

func extractMentionIdentifiers(body string) []string {
	identifiers := map[string]struct{}{}
	for _, pattern := range mentionIdentifierPatterns {
		matches := pattern.FindAllStringSubmatch(body, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			identifiers[strings.TrimSpace(match[1])] = struct{}{}
		}
	}

	return mapKeys(identifiers)
}

func getCommentMentionSource(comment *CommentResponse) string {
	if comment == nil {
		return ""
	}

	body := strings.TrimSpace(comment.Body.Storage.Value)
	if body != "" {
		return body
	}

	return strings.TrimSpace(comment.Body.View.Value)
}

func getCommentExcerpt(comment *CommentResponse) string {
	if comment == nil {
		return ""
	}

	if excerpt := strings.TrimSpace(util.GetBodyForExcerpt(comment.Body.View.Value)); excerpt != "" {
		return excerpt
	}

	return strings.TrimSpace(util.GetBodyForExcerpt(comment.Body.Storage.Value))
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	return keys
}
