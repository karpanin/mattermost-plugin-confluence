package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	stderrors "errors"
	errors "github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

var confluenceServerWebhook = &Endpoint{
	Path:            "/server/webhook",
	Method:          http.MethodPost,
	Execute:         handleConfluenceServerWebhook,
	IsAuthenticated: false,
}

func handleConfluenceServerWebhook(w http.ResponseWriter, r *http.Request, p *Plugin) {
	p.client.Log.Info("Received Confluence server event.")

	if status, err := verifyHTTPSecret(config.GetConfig().Secret, r.FormValue("secret")); err != nil {
		p.client.Log.Error("Error verifying secret for the Confluence server webhook", "error", err.Error())
		http.Error(w, "Failed to verify secret for the Confluence server webhook", status)
		return
	}

	pluginConfig := config.GetConfig()

	if pluginConfig.ServerVersionGreaterthan9 {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			p.client.Log.Error("Error reading body of the Confluence server webhook", "error", err.Error())
			http.Error(w, "Failed to read body for the Confluence server webhook", http.StatusBadRequest)
			return
		}

		if respondToTestConnection(body) {
			w.Header().Set("Content-Type", "application/json")
			ReturnStatusOK(w)
			return
		}

		if err := p.processConfluenceServerWebhook(body); err != nil {
			p.client.Log.Error("Failed to process Confluence server webhook", "error", err.Error())
			http.Error(w, "Failed to process Confluence server webhook", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		ReturnStatusOK(w)
	} else {
		event, err := serializer.ConfluenceServerEventFromJSON(r.Body)
		if err != nil {
			p.client.Log.Error("Error occurred while unmarshalling Confluence server webhook payload", "error", err)
			http.Error(w, "Failed to unmarshal Confluence server webhook payload", http.StatusInternalServerError)
			return
		}

		go service.SendConfluenceNotifications(event, event.Event)
	}
}

func (p *Plugin) processConfluenceServerWebhook(body []byte) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic while processing Confluence server webhook: %v", recovered)
		}
	}()

	var event *serializer.ConfluenceServerWebhookPayload
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("error unmarshalling Confluence server webhook payload: %w", err)
	}

	p.client.Log.Info("Processing Confluence server webhook event",
		"event", event.Event,
		"user_key", event.UserKey,
		"comment_id", event.Comment.ID,
		"page_id", event.Page.ID,
		"space_id", event.Space.ID,
		"space_key", event.Space.SpaceKey,
	)

	if !isSupportedServerWebhookEvent(event.Event) {
		p.client.Log.Info("Skipping unsupported Confluence server webhook event",
			"event", event.Event,
			"user_key", event.UserKey,
			"comment_id", event.Comment.ID,
			"page_id", event.Page.ID,
			"space_id", event.Space.ID,
			"space_key", event.Space.SpaceKey,
		)
		return nil
	}

	pluginConfig := config.GetConfig()
	instanceID := pluginConfig.ConfluenceURL
	notification := p.getNotification()

	client, _, err := p.GetClientFromUserKey(instanceID, event.UserKey)
	// If there is an error while retrieving the client from the event user key, it could be due to one of the following reasons:
	// - An expected error occurred.
	// - The user who triggered the event in Confluence is not connected to Mattermost.
	// If the Admin API token is available, we will attempt to fetch additional data using it to send a detailed notification.
	// Otherwise, a generic notification will be sent.
	if err != nil {
		if pluginConfig.AdminAPIToken != "" {
			p.client.Log.Info("Error getting client for the user who triggered webhook event. Sending notification using admin API token")
			if strings.Contains(event.Event, Space) {
				spaceKey, spaceErr := p.GetSpaceKeyFromSpaceIDWithAPIToken(event.Space.ID, pluginConfig)
				if spaceErr != nil {
					return fmt.Errorf("error getting space key using space ID with API token: %w", spaceErr)
				}
				event.Space.SpaceKey = spaceKey
			}

			eventData, eventErr := p.GetEventDataWithAPIToken(event, pluginConfig)
			if eventErr != nil {
				return fmt.Errorf("error getting event data with API token: %w", eventErr)
			}

			eventTriggerer, triggerErr := p.GetUserFromUserKeyWithAPIToken(event.UserKey, pluginConfig)
			if triggerErr != nil {
				return fmt.Errorf("error getting details of the event triggerer user using API token: %w", triggerErr)
			}

			eventData.BaseURL = pluginConfig.ConfluenceURL
			notification.SendConfluenceNotifications(eventData, event.Event, p.BotUserID, eventTriggerer.DisplayName, event.UserKey)
			return nil
		}

		p.client.Log.Info("Error getting client for the user who triggered webhook event. Sending generic notification")
		notification.SendGenericWHNotification(event, p.BotUserID, pluginConfig.ConfluenceURL)
		return nil
	}

	if strings.Contains(event.Event, Space) {
		spaceKey, spaceErr := client.(*confluenceServerClient).GetSpaceKeyFromSpaceID(event.Space.ID)
		if spaceErr != nil {
			return fmt.Errorf("failed to get space key from the space ID %d: %w", event.Space.ID, spaceErr)
		}
		event.Space.SpaceKey = spaceKey
	}

	eventData, eventErr := p.GetEventData(event, client)
	if eventErr != nil {
		return fmt.Errorf("error getting event data for the Confluence server webhook: %w", eventErr)
	}

	eventData.BaseURL = pluginConfig.ConfluenceURL

	// Prefer Admin API Token if available since regular user tokens lack this permission.
	var eventTriggerer *ConfluenceUser
	if pluginConfig.AdminAPIToken != "" {
		eventTriggerer, err = p.GetUserFromUserKeyWithAPIToken(event.UserKey, pluginConfig)
		if err != nil {
			return fmt.Errorf("error getting details of the event triggerer user using API token: %w", err)
		}
	} else {
		// Fallback to user's OAuth token if Admin API Token is not configured
		eventTriggerer, err = client.(*confluenceServerClient).GetUserFromUserKey(event.UserKey)
		if err != nil {
			return fmt.Errorf("error getting details of the event triggerer user: %w", err)
		}
	}

	notification.SendConfluenceNotifications(eventData, event.Event, p.BotUserID, eventTriggerer.DisplayName, event.UserKey)
	return nil
}

func isSupportedServerWebhookEvent(eventType string) bool {
	switch eventType {
	case
		serializer.PageCreatedEvent,
		serializer.PageUpdatedEvent,
		serializer.PageTrashedEvent,
		serializer.PageRestoredEvent,
		serializer.PageRemovedEvent,
		serializer.CommentCreatedEvent,
		serializer.CommentUpdatedEvent,
		serializer.SpaceUpdatedEvent:
		return true
	default:
		return false
	}
}

func (p *Plugin) GetEventData(webhookPayload *serializer.ConfluenceServerWebhookPayload, client Client) (*ConfluenceServerEvent, error) {
	eventData, err := client.(*confluenceServerClient).GetEventData(webhookPayload)
	if err != nil {
		p.API.LogError("Error occurred while fetching event data.", "Error", err.Error())
		return nil, err
	}

	return eventData, nil
}

func (p *Plugin) GetClientFromUserKey(instanceID, eventUserKey string) (Client, *string, error) {
	mmUserID, err := store.GetMattermostUserIDFromConfluenceID(instanceID, eventUserKey)
	if err != nil {
		mmUserID, err = p.tryRecoverMattermostUserIDFromConfluenceUser(instanceID, eventUserKey)
		if err != nil {
			if stderrors.Is(err, store.ErrNotFound) || errors.Cause(err) == store.ErrNotFound {
				p.client.Log.Info("No Mattermost user mapping found for Confluence user", "InstanceID", instanceID, "Confluence Account ID", eventUserKey)
			} else {
				p.client.Log.Error("Error getting Mattermost User ID from Confluence ID", "InstanceID", instanceID, "Confluence Account ID", eventUserKey, "error", err.Error())
			}
			return nil, nil, err
		}
	}

	connection, err := store.LoadConnection(instanceID, *mmUserID)
	if err != nil {
		p.client.Log.Error("Error loading the connection", "UserID", *mmUserID, "InstanceURL", instanceID, "error", err.Error())
		return nil, nil, err
	}

	client, err := p.GetServerClient(instanceID, connection)
	if err != nil {
		p.client.Log.Error("Error getting server client", "InstanceID", instanceID, "error", err.Error())
		return nil, nil, err
	}

	return client, mmUserID, nil
}

func (p *Plugin) tryRecoverMattermostUserIDFromConfluenceUser(instanceID, eventUserKey string) (*string, error) {
	pluginConfig := config.GetConfig()
	if pluginConfig.AdminAPIToken == "" {
		return nil, store.ErrNotFound
	}

	confluenceUser, err := p.GetUserFromUserKeyWithAPIToken(eventUserKey, pluginConfig)
	if err != nil {
		return nil, err
	}

	if confluenceUser == nil || strings.TrimSpace(confluenceUser.Username) == "" {
		return nil, store.ErrNotFound
	}

	mmUserID, err := store.GetMattermostUserIDFromConfluenceID(instanceID, confluenceUser.Username)
	if err != nil {
		return nil, err
	}

	connection, err := store.LoadConnection(instanceID, *mmUserID)
	if err != nil {
		return nil, err
	}

	if connection.AccountID == "" {
		connection.AccountID = eventUserKey
	}
	if connection.Name == "" {
		connection.Name = confluenceUser.Username
	}

	if storeErr := store.StoreConnection(instanceID, *mmUserID, connection); storeErr != nil {
		return nil, storeErr
	}

	p.client.Log.Info("Recovered Mattermost user mapping for Confluence user", "InstanceID", instanceID, "Confluence Account ID", eventUserKey, "Mattermost User ID", *mmUserID)
	return mmUserID, nil
}

func (p *Plugin) GetUserFromUserKeyWithAPIToken(eventUserKey string, pluginConfig *config.Configuration) (*ConfluenceUser, error) {
	var user ConfluenceUser

	path := fmt.Sprintf("%s%s?key=%s", pluginConfig.ConfluenceURL, PathUserData, eventUserKey)

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching user data with API token: %w", err)
	}

	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("error unmarshaling user data with API token: %w", err)
	}

	return &user, nil
}
func (p *Plugin) GetSpaceKeyFromSpaceIDWithAPIToken(spaceID int64, pluginConfig *config.Configuration) (string, error) {
	start := 0

	for {
		path := fmt.Sprintf("%s%s?start=%d&limit=%d", pluginConfig.ConfluenceURL, PathSpaceData, start, pageSize)

		response := &apiResponse{}

		body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
		if err != nil || statusCode != http.StatusOK {
			return "", errors.Wrapf(err, "error getting spaceKey from spaceID")
		}

		if err = json.Unmarshal(body, response); err != nil {
			return "", errors.Wrapf(err, "failed to unmarshal spaceKey data")
		}

		for _, space := range response.Results {
			if space.ID == spaceID {
				return space.Key, nil
			}
		}

		if len(response.Results) < pageSize {
			break
		}

		start += pageSize
	}

	return "", fmt.Errorf("confluence GetSpaceKeyFromSpaceIDUsingAPIToken: no space found for the space key")
}

func (p *Plugin) GetEventDataWithAPIToken(webhookPayload *serializer.ConfluenceServerWebhookPayload, pluginConfig *config.Configuration) (*ConfluenceServerEvent, error) {
	var confluenceServerEvent ConfluenceServerEvent
	var err error
	supportedWHEventFound := false

	if strings.Contains(webhookPayload.Event, Comment) {
		supportedWHEventFound = true
		confluenceServerEvent.Comment, err = p.GetCommentDataWithAPIToken(webhookPayload, pluginConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting comment data for the event using API token")
		}
	}

	if strings.Contains(webhookPayload.Event, Page) {
		supportedWHEventFound = true
		confluenceServerEvent.Page, err = p.GetPageDataWithAPIToken(int(webhookPayload.Page.ID), pluginConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting page data for the event using API token")
		}
	}

	if strings.Contains(webhookPayload.Event, Space) {
		supportedWHEventFound = true
		confluenceServerEvent.Space, err = p.GetSpaceDataWithAPIToken(webhookPayload.Space.SpaceKey, pluginConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting space data for the event using API token")
		}
	}

	if !supportedWHEventFound {
		return nil, errors.New("unable to get data for unsupported webhook event")
	}

	return &confluenceServerEvent, nil
}

func (p *Plugin) GetCommentDataWithAPIToken(webhookPayload *serializer.ConfluenceServerWebhookPayload, pluginConfig *config.Configuration) (*CommentResponse, error) {
	commentID := strconv.FormatInt(webhookPayload.Comment.ID, 10)

	var lastErr error
	for _, delay := range []time.Duration{0, 250 * time.Millisecond, 750 * time.Millisecond} {
		if delay > 0 {
			time.Sleep(delay)
		}

		commentResponse := &CommentResponse{}
		path := fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, fmt.Sprintf("%s%s?status=any&expand=body.view,body.storage,container,space,history", PathContentData, commentID))

		body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
		if err == nil && statusCode == http.StatusOK {
			if unmarshalErr := json.Unmarshal(body, commentResponse); unmarshalErr != nil {
				return nil, errors.Wrapf(unmarshalErr, "error getting comment data with API token")
			}
			return commentResponse, nil
		}

		if statusCode == http.StatusNotFound {
			if webhookPayload.Page.ID != 0 {
				p.client.Log.Info("Comment lookup by content ID returned 404, trying page descendants fallback",
					"comment_id", webhookPayload.Comment.ID,
					"page_id", webhookPayload.Page.ID,
					"event", webhookPayload.Event,
				)
				comment, descendantErr := p.GetCommentDataFromPageDescendantsWithAPIToken(strconv.FormatInt(webhookPayload.Page.ID, 10), commentID, pluginConfig)
				if descendantErr == nil {
					return comment, nil
				}
				lastErr = descendantErr
				continue
			}

			p.client.Log.Info("Comment lookup by content ID returned 404 without page ID, trying content search fallback",
				"comment_id", webhookPayload.Comment.ID,
				"event", webhookPayload.Event,
			)
			comment, searchErr := p.SearchCommentDataByIDWithAPIToken(commentID, pluginConfig)
			if searchErr == nil {
				return comment, nil
			}
			lastErr = searchErr
			continue
		}

		if err == nil {
			lastErr = fmt.Errorf("unexpected status code %d while fetching comment %d with API token", statusCode, webhookPayload.Comment.ID)
		} else {
			lastErr = err
		}
	}

	return nil, lastErr
}

func (p *Plugin) GetCommentDataFromPageDescendantsWithAPIToken(pageID, commentID string, pluginConfig *config.Configuration) (*CommentResponse, error) {
	const descendantPageSize = 200

	for start := 0; ; start += descendantPageSize {
		response := &DescendantCommentSearchResponse{}
		path := fmt.Sprintf("%s%s/descendant/comment?expand=body.view,body.storage,container,space,history&start=%d&limit=%d", PathContentData, pageID, start, descendantPageSize)
		body, statusCode, err := p.MakeHTTPCallWithAPIToken(fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, path))
		if err != nil || statusCode != http.StatusOK {
			return nil, err
		}

		if err := json.Unmarshal(body, response); err != nil {
			return nil, errors.Wrapf(err, "error getting descendant comment data with API token")
		}

		comments := getDescendantComments(response)
		for i := range comments {
			if comments[i].ID == commentID {
				return &comments[i], nil
			}
		}

		if len(comments) < descendantPageSize {
			break
		}
	}

	return nil, errors.Errorf("comment %s not found in descendants for page %s", commentID, pageID)
}

func (p *Plugin) SearchCommentDataByIDWithAPIToken(commentID string, pluginConfig *config.Configuration) (*CommentResponse, error) {
	response := &CommentSearchResponse{}
	path := fmt.Sprintf("%s%s?cql=%s&expand=body.view,body.storage,container,space,history&limit=1", pluginConfig.ConfluenceURL, PathContentData+"search", url.QueryEscape("id="+commentID))
	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		if err == nil {
			return nil, fmt.Errorf("unexpected status code %d while searching comment %s with API token", statusCode, commentID)
		}
		return nil, err
	}

	if err := json.Unmarshal(body, response); err != nil {
		return nil, errors.Wrapf(err, "error searching comment data with API token")
	}

	for i := range response.Results {
		if response.Results[i].ID == commentID {
			return &response.Results[i], nil
		}
	}

	return nil, errors.Errorf("comment %s not found via content search", commentID)
}

func (p *Plugin) GetPageDataWithAPIToken(pageID int, pluginConfig *config.Configuration) (*PageResponse, error) {
	pageResponse := &PageResponse{}
	path := fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, fmt.Sprintf("%s%s?status=any&expand=body.view,body.storage,container,space,history.previousVersion,version", PathContentData, strconv.Itoa(pageID)))

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, err
	}

	if err := json.Unmarshal(body, pageResponse); err != nil {
		return nil, errors.Wrapf(err, "error getting page data with API token")
	}

	return pageResponse, nil
}

func (p *Plugin) GetPreviousPageVersionWithAPIToken(pageID string, currentVersion int, pluginConfig *config.Configuration) (*PageResponse, error) {
	if currentVersion <= 1 {
		return nil, nil
	}

	previousVersion := currentVersion - 1
	pageResponse := &PageResponse{}

	primaryPath := fmt.Sprintf("%s%s%s?status=historical&version=%d&expand=body.view,body.storage,space,history.previousVersion,version", pluginConfig.ConfluenceURL, PathContentData, pageID, previousVersion)
	body, statusCode, err := p.MakeHTTPCallWithAPIToken(primaryPath)
	if err == nil && statusCode == http.StatusOK {
		if err := json.Unmarshal(body, pageResponse); err != nil {
			return nil, errors.Wrapf(err, "error getting historical page data with API token")
		}
		return pageResponse, nil
	}

	response := &PageVersionResponse{}
	fallbackPath := fmt.Sprintf("%s%s%s/version/%d?expand=content.body.view,content.body.storage,content.space,content.history.previousVersion,content.version", pluginConfig.ConfluenceURL, PathContentData, pageID, previousVersion)
	body, statusCode, err = p.MakeHTTPCallWithAPIToken(fallbackPath)
	if err != nil || statusCode != http.StatusOK {
		if err == nil {
			return nil, fmt.Errorf("unexpected status code %d while fetching previous page version %d with API token", statusCode, previousVersion)
		}
		return nil, err
	}

	if err := json.Unmarshal(body, response); err != nil {
		return nil, errors.Wrapf(err, "error getting previous page version data with API token")
	}

	return &response.Content, nil
}

func (p *Plugin) GetSpaceDataWithAPIToken(spaceKey string, pluginConfig *config.Configuration) (*SpaceResponse, error) {
	spaceResponse := &SpaceResponse{}
	path := fmt.Sprintf("%s%s", pluginConfig.ConfluenceURL, fmt.Sprintf("%s%s?status=any", PathSpaceData, spaceKey))

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, err
	}

	if err := json.Unmarshal(body, spaceResponse); err != nil {
		return nil, errors.Wrapf(err, "error getting space data with APIToken")
	}

	return spaceResponse, nil
}

func (p *Plugin) MakeHTTPCallWithAPIToken(path string) ([]byte, int, error) {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	err = p.SetAdminAPITokenRequestHeader(req)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	if resp == nil || resp.Body == nil {
		return nil, http.StatusInternalServerError, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return body, resp.StatusCode, err
}

func (p *Plugin) GetContentWatchersWithAPIToken(pageID string, pluginConfig *config.Configuration) ([]ConfluenceWatcher, error) {
	path := fmt.Sprintf("%s%s%s/watchers", pluginConfig.ConfluenceURL, PathContentData, pageID)

	body, statusCode, err := p.MakeHTTPCallWithAPIToken(path)
	if err != nil || statusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting content watchers with API token: %w", err)
	}

	response := &ContentWatchersResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("error unmarshalling content watchers with API token: %w", err)
	}

	return response.Results, nil
}

func (p *Plugin) SetAdminAPITokenRequestHeader(req *http.Request) error {
	pluginConfig := config.GetConfig()

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pluginConfig.AdminAPIToken))
	req.Header.Set("Accept", "application/json")

	return nil
}

func respondToTestConnection(body []byte) bool {
	var testConnectionBody struct {
		Test bool `json:"test"`
	}

	if err := json.Unmarshal(body, &testConnectionBody); err != nil {
		return false
	}

	return testConnectionBody.Test
}
