package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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

	if eventType == serializer.PageCreatedEvent || eventType == serializer.PageUpdatedEvent {
		for _, userID := range n.resolvePageMentionedUserIDs(baseURL, event, eventType, eventTriggererKey) {
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

func (n *notification) resolvePageMentionedUserIDs(instanceID string, event serializer.ConfluenceEventV2, eventType, eventTriggererKey string) []string {
	serverEvent, ok := event.(*ConfluenceServerEvent)
	if !ok || serverEvent.Page == nil {
		return nil
	}

	currentUserIDs := n.resolveMattermostUserIDsFromMentionSource(instanceID, getPageMentionSource(serverEvent.Page), eventTriggererKey)
	if eventType == serializer.PageCreatedEvent && len(currentUserIDs) == 0 {
		refetchedPage, err := n.refetchCurrentPageForMentions(instanceID, serverEvent, eventTriggererKey)
		if err != nil {
			n.API.LogError("Unable to refetch created page for mention notifications", "PageID", serverEvent.Page.ID, "Error", err.Error())
		} else if refetchedPage != nil {
			currentUserIDs = n.resolveMattermostUserIDsFromMentionSource(instanceID, getPageMentionSource(refetchedPage), eventTriggererKey)
			serverEvent.Page = refetchedPage
		}
	}
	if len(currentUserIDs) == 0 {
		return nil
	}

	if eventType == serializer.PageUpdatedEvent {
		previousPage, err := n.getPreviousPageForMentions(instanceID, serverEvent, eventTriggererKey)
		if err != nil {
			n.API.LogError("Unable to get previous page version for page mention notifications", "PageID", serverEvent.Page.ID, "Error", err.Error())
			return nil
		}
		if previousPage == nil && serverEvent.Page.Version.Number > 1 {
			return nil
		}

		previousUserIDs := map[string]struct{}{}
		if previousPage != nil {
			previousUserIDs = n.resolveMattermostUserIDsFromMentionSource(instanceID, getPageMentionSource(previousPage), eventTriggererKey)
		}

		newUserIDs := map[string]struct{}{}
		for userID := range currentUserIDs {
			if _, exists := previousUserIDs[userID]; !exists {
				newUserIDs[userID] = struct{}{}
			}
		}
		return mapKeys(newUserIDs)
	}

	return mapKeys(currentUserIDs)
}

func (n *notification) getPreviousPageForMentions(instanceID string, event *ConfluenceServerEvent, eventTriggererKey string) (*PageResponse, error) {
	if event == nil || event.Page == nil || event.Page.Version.Number <= 1 {
		return nil, nil
	}

	var lastErr error
	for _, delay := range []time.Duration{0, 350 * time.Millisecond, 1200 * time.Millisecond} {
		if delay > 0 {
			time.Sleep(delay)
		}

		page, err := n.fetchPreviousPageForMentions(instanceID, event.Page.ID, event.Page.Version.Number, eventTriggererKey)
		if err == nil {
			return page, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

func (n *notification) refetchCurrentPageForMentions(instanceID string, event *ConfluenceServerEvent, eventTriggererKey string) (*PageResponse, error) {
	if event == nil || event.Page == nil || event.Page.ID == "" {
		return nil, nil
	}

	delays := []time.Duration{250 * time.Millisecond, 1 * time.Second}
	for _, delay := range delays {
		time.Sleep(delay)

		page, err := n.fetchCurrentPageForMentions(instanceID, event.Page.ID, eventTriggererKey)
		if err != nil {
			continue
		}
		if page == nil {
			continue
		}
		if len(extractMentionIdentifiers(getPageMentionSource(page))) > 0 {
			return page, nil
		}
	}

	return nil, nil
}

func (n *notification) fetchCurrentPageForMentions(instanceID, pageID, eventTriggererKey string) (*PageResponse, error) {
	if client, _, err := n.GetClientFromUserKey(instanceID, eventTriggererKey); err == nil {
		pageIDInt, convErr := strconv.Atoi(pageID)
		if convErr != nil {
			return nil, convErr
		}
		return client.(*confluenceServerClient).GetPageData(pageIDInt)
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.AdminAPIToken == "" {
		return nil, nil
	}

	pageIDInt, err := strconv.Atoi(pageID)
	if err != nil {
		return nil, err
	}
	return n.GetPageDataWithAPIToken(pageIDInt, pluginConfig)
}

func (n *notification) fetchPreviousPageForMentions(instanceID, pageID string, currentVersion int, eventTriggererKey string) (*PageResponse, error) {
	if client, _, err := n.GetClientFromUserKey(instanceID, eventTriggererKey); err == nil {
		return client.(*confluenceServerClient).GetPreviousPageVersion(pageID, currentVersion)
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.AdminAPIToken == "" {
		return nil, nil
	}

	return n.GetPreviousPageVersionWithAPIToken(pageID, currentVersion, pluginConfig)
}

func (n *notification) resolveMattermostUserIDsFromMentionSource(instanceID, body, eventTriggererKey string) map[string]struct{} {
	mmUserIDs := map[string]struct{}{}
	for _, identifier := range extractMentionIdentifiers(body) {
		if identifier == "" || identifier == eventTriggererKey {
			continue
		}

		mmUserID, err := store.GetMattermostUserIDFromConfluenceID(instanceID, identifier)
		if err != nil || mmUserID == nil || *mmUserID == "" {
			continue
		}
		mmUserIDs[*mmUserID] = struct{}{}
	}

	return mmUserIDs
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
		if serverEvent.Comment != nil {
			pageName := serverEvent.GetPageDisplayNameForCommentEvents(baseURL)
			spaceName := serverEvent.GetSpaceDisplayNameForCommentEvents(baseURL)
			commentURL := joinURL(baseURL, serverEvent.Comment.Links.Self)
			commentExcerpt := getCommentExcerpt(serverEvent.Comment)
			if commentExcerpt != "" {
				return fmt.Sprintf("%s mentioned you in a [comment](%s) on %s in %s.\n> %s", eventTriggerer, commentURL, pageName, spaceName, strings.ReplaceAll(commentExcerpt, "\n", "\n> "))
			}
			return fmt.Sprintf("%s mentioned you in a [comment](%s) on %s in %s.", eventTriggerer, commentURL, pageName, spaceName)
		}

		if serverEvent.Page != nil && (eventType == serializer.PageCreatedEvent || eventType == serializer.PageUpdatedEvent) {
			spaceName := serverEvent.GetSpaceDisplayNameForPageEvents(baseURL)
			pageURL := joinURL(baseURL, serverEvent.Page.Links.Self)
			return fmt.Sprintf("%s mentioned you on [this page](%s) in %s.", eventTriggerer, pageURL, spaceName)
		}

		return ""
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

func getPageMentionSource(page *PageResponse) string {
	if page == nil {
		return ""
	}

	body := strings.TrimSpace(page.Body.Storage.Value)
	if body != "" {
		return body
	}

	return strings.TrimSpace(page.Body.View.Value)
}

func getPageExcerpt(page *PageResponse) string {
	if page == nil {
		return ""
	}

	if excerpt := strings.TrimSpace(util.GetBodyForExcerpt(page.Body.View.Value)); excerpt != "" {
		return excerpt
	}

	return strings.TrimSpace(util.GetBodyForExcerpt(page.Body.Storage.Value))
}

func mapKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	return keys
}
