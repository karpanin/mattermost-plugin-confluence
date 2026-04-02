package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
)

var createPageFromPost = &Endpoint{
	Path:            "/page/from-post",
	Method:          http.MethodPost,
	Execute:         handleCreatePageFromPost,
	IsAuthenticated: true,
}

type CreatePageFromPostRequest struct {
	PostID       string `json:"postID"`
	SpaceKey     string `json:"spaceKey"`
	Title        string `json:"title"`
	ParentPageID string `json:"parentPageID"`
}

type CreatePageFromPostResponse struct {
	PageID   string `json:"pageID"`
	Title    string `json:"title"`
	PageURL  string `json:"pageURL"`
	ThreadID string `json:"threadID"`
}

func handleCreatePageFromPost(w http.ResponseWriter, r *http.Request, p *Plugin) {
	userID := r.Header.Get(config.HeaderMattermostUserID)
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	pluginConfig := config.GetConfig()
	if pluginConfig.ConfluenceURL == "" {
		http.Error(w, "Confluence is not configured.", http.StatusInternalServerError)
		return
	}

	req := &CreatePageFromPostRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "Could not decode request body.", http.StatusBadRequest)
		return
	}

	req.PostID = strings.TrimSpace(req.PostID)
	req.SpaceKey = strings.TrimSpace(req.SpaceKey)
	req.Title = strings.TrimSpace(req.Title)
	req.ParentPageID = strings.TrimSpace(req.ParentPageID)

	if req.PostID == "" || req.SpaceKey == "" || req.Title == "" {
		http.Error(w, "postID, spaceKey and title are required.", http.StatusBadRequest)
		return
	}

	post, appErr := p.API.GetPost(req.PostID)
	if appErr != nil || post == nil {
		http.Error(w, "Original Mattermost post not found.", http.StatusNotFound)
		return
	}

	if !p.hasChannelAccess(userID, post.ChannelId) {
		http.Error(w, "User does not have access to this post.", http.StatusForbidden)
		return
	}

	connection, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "User not connected. Please use `/confluence connect`.", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Unable to verify user's Confluence connection.", http.StatusInternalServerError)
		return
	}

	if connection.ConfluenceAccountID() == "" {
		http.Error(w, "User not connected. Please use `/confluence connect`.", http.StatusUnauthorized)
		return
	}

	client, err := p.GetServerClient(pluginConfig.ConfluenceURL, connection)
	if err != nil {
		http.Error(w, "Failed to create Confluence client.", http.StatusInternalServerError)
		return
	}

	if _, err = client.GetSpaceData(req.SpaceKey); err != nil {
		http.Error(w, "User does not have access to this Confluence space.", http.StatusForbidden)
		return
	}

	if req.ParentPageID != "" {
		parentPageID, convErr := strconv.Atoi(req.ParentPageID)
		if convErr != nil {
			http.Error(w, "parentPageID should be numeric.", http.StatusBadRequest)
			return
		}

		if _, err = client.GetPageData(parentPageID); err != nil {
			http.Error(w, "User does not have access to this parent Confluence page.", http.StatusForbidden)
			return
		}
	}

	permalink := getMattermostPermalink(req.PostID)
	createdPage, err := client.CreatePage(&CreatePageInput{
		Title:        req.Title,
		SpaceKey:     req.SpaceKey,
		ParentPageID: req.ParentPageID,
		Body:         formatMattermostPostForConfluence(post.Message, permalink),
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = "Failed to create Confluence page."
		}

		lowerMessage := strings.ToLower(message)
		if strings.Contains(lowerMessage, "already exists") || strings.Contains(lowerMessage, "same title") {
			statusCode = http.StatusConflict
			message = fmt.Sprintf("A Confluence page with the title %q already exists in space %q. Please choose a different title.", req.Title, req.SpaceKey)
		}

		http.Error(w, message, statusCode)
		return
	}

	pageURL := joinURL(pluginConfig.ConfluenceURL, createdPage.Links.Self)
	threadID := post.Id
	if post.RootId != "" {
		threadID = post.RootId
	}

	if err = saveLastSelectedSpaceForUser(userID, req.SpaceKey); err != nil {
		p.client.Log.Error("Failed to save last selected Confluence space", "user_id", userID, "space_key", req.SpaceKey, "error", err.Error())
	}

	if err = publishThreadPost(p, userID, post.ChannelId, threadID, fmt.Sprintf("Created Confluence page: [%s](%s)", createdPage.Title, pageURL)); err != nil {
		p.client.Log.Error("Confluence page was created, but publishing the Mattermost thread post failed", "user_id", userID, "thread_id", threadID, "page_id", createdPage.ID, "error", err.Error())
	}

	_ = p.API.SendEphemeralPost(userID, &model.Post{
		UserId:    config.BotUserID,
		ChannelId: post.ChannelId,
		RootId:    threadID,
		Message:   fmt.Sprintf("Created Confluence page: [%s](%s)", createdPage.Title, pageURL),
	})

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(CreatePageFromPostResponse{
		PageID:   createdPage.ID,
		Title:    createdPage.Title,
		PageURL:  pageURL,
		ThreadID: threadID,
	}); err != nil {
		http.Error(w, "Failed to encode response.", http.StatusInternalServerError)
	}
}
