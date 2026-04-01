package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

var addCommentToPageFromPost = &Endpoint{
	Path:            "/page/comment-from-post",
	Method:          http.MethodPost,
	Execute:         handleAddCommentToPageFromPost,
	IsAuthenticated: true,
}

type AddCommentToPageFromPostRequest struct {
	PostID   string `json:"postID"`
	PageID   string `json:"pageID"`
	SpaceKey string `json:"spaceKey"`
}

type AddCommentToPageFromPostResponse struct {
	PageID     string `json:"pageID"`
	PageTitle  string `json:"pageTitle"`
	PageURL    string `json:"pageURL"`
	CommentID  string `json:"commentID"`
	CommentURL string `json:"commentURL"`
	ThreadID   string `json:"threadID"`
}

func handleAddCommentToPageFromPost(w http.ResponseWriter, r *http.Request, p *Plugin) {
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

	req := &AddCommentToPageFromPostRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "Could not decode request body.", http.StatusBadRequest)
		return
	}

	req.PostID = strings.TrimSpace(req.PostID)
	req.PageID = strings.TrimSpace(req.PageID)
	req.SpaceKey = strings.TrimSpace(req.SpaceKey)
	if req.PostID == "" || req.PageID == "" {
		http.Error(w, "postID and pageID are required.", http.StatusBadRequest)
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

	pageID, convErr := strconv.Atoi(req.PageID)
	if convErr != nil {
		http.Error(w, "pageID should be numeric.", http.StatusBadRequest)
		return
	}

	page, err := client.GetPageData(pageID)
	if err != nil {
		http.Error(w, "User does not have access to this Confluence page.", http.StatusForbidden)
		return
	}

	permalink := getMattermostPermalink(req.PostID)
	createdComment, err := client.AddCommentToPage(&CreateCommentInput{
		PageID: req.PageID,
		Body:   formatMattermostPostForConfluence(post.Message, permalink),
	})
	if err != nil {
		http.Error(w, "Failed to add Confluence comment.", http.StatusInternalServerError)
		return
	}

	pageURL := joinURL(pluginConfig.ConfluenceURL, page.Links.Self)
	commentURL := joinURL(pluginConfig.ConfluenceURL, createdComment.Links.Self)
	threadID := post.Id
	if post.RootId != "" {
		threadID = post.RootId
	}

	spaceKey := req.SpaceKey
	if spaceKey == "" {
		spaceKey = page.Space.Key
	}
	if err = saveLastSelectedSpaceForUser(userID, spaceKey); err != nil {
		p.client.Log.Error("Failed to save last selected Confluence space", "user_id", userID, "space_key", spaceKey, "error", err.Error())
	}

	if err = publishThreadPost(p, userID, post.ChannelId, threadID, fmt.Sprintf("Added Confluence comment to [%s](%s)", page.Title, pageURL)); err != nil {
		http.Error(w, "Confluence comment was created, but publishing the Mattermost thread post failed.", http.StatusInternalServerError)
		return
	}

	_ = p.API.SendEphemeralPost(userID, &model.Post{
		UserId:    config.BotUserID,
		ChannelId: post.ChannelId,
		RootId:    threadID,
		Message:   fmt.Sprintf("Added Confluence comment to [%s](%s)", page.Title, pageURL),
	})

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(AddCommentToPageFromPostResponse{
		PageID:     page.ID,
		PageTitle:  page.Title,
		PageURL:    pageURL,
		CommentID:  createdComment.ID,
		CommentURL: commentURL,
		ThreadID:   threadID,
	}); err != nil {
		http.Error(w, "Failed to encode response.", http.StatusInternalServerError)
	}
}

func publishThreadPost(p *Plugin, userID, channelID, threadID, message string) error {
	post := &model.Post{
		UserId:    userID,
		ChannelId: channelID,
		RootId:    threadID,
		Message:   message,
	}

	if _, appErr := p.API.CreatePost(post); appErr != nil {
		return errors.New(appErr.Error())
	}

	return nil
}

func getMattermostPermalink(postID string) string {
	return strings.TrimRight(util.GetSiteURL(), "/") + "/_redirect/pl/" + postID
}
