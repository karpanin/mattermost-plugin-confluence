package main

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/mattermost/mattermost-plugin-confluence/server/serializer"
	"github.com/mattermost/mattermost-plugin-confluence/server/service"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const (
	PathCurrentUser = "/rest/api/user/current"
	PathContentData = "/rest/api/content/"
	PathSpaceData   = "/rest/api/space/"
	PathUserData    = "/rest/api/user/"
	PathAdminData   = "/rest/api/audit"
)

const (
	Comment = "comment"
	Space   = "space"
	Page    = "page"
)

const pageSize = 10

type confluenceServerClient struct {
	URL        string
	HTTPClient *http.Client
}

type ConfluenceServerUser struct {
	UserKey     string `json:"userKey"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

type AdminData struct {
	Number int    `json:"number"`
	Units  string `json:"units"`
}

type SpaceResponse struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Links Links  `json:"_links"`
}

type CommentContainer struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Links Links  `json:"_links"`
}

type Links struct {
	Self string `json:"webui"`
}

type View struct {
	Value string `json:"value"`
}

type Body struct {
	View    View `json:"view"`
	Storage View `json:"storage"`
}

type CreatedBy struct {
	Username string `json:"username"`
}

type History struct {
	CreatedBy CreatedBy `json:"createdBy"`
}

type ContentBodyStorage struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

type ContentBodyPayload struct {
	Storage ContentBodyStorage `json:"storage"`
}

type ContentSpacePayload struct {
	Key string `json:"key"`
}

type ContentAncestorPayload struct {
	ID string `json:"id"`
}

type CommentResponse struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Space     SpaceResponse    `json:"space"`
	Container CommentContainer `json:"container"`
	Body      Body             `json:"body"`
	Links     Links            `json:"_links"`
	History   History          `json:"history"`
}

type PageResponse struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Space   SpaceResponse `json:"space"`
	Body    Body          `json:"body"`
	Links   Links         `json:"_links"`
	History History       `json:"history"`
}

type CreatePageInput struct {
	Title        string
	SpaceKey     string
	ParentPageID string
	Body         string
}

type CreateCommentInput struct {
	PageID string
	Body   string
}

type CreatePagePayload struct {
	Type      string                   `json:"type"`
	Title     string                   `json:"title"`
	Space     ContentSpacePayload      `json:"space"`
	Body      ContentBodyPayload       `json:"body"`
	Ancestors []ContentAncestorPayload `json:"ancestors,omitempty"`
}

type ContentContainerPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type CreateCommentPayload struct {
	Type      string                  `json:"type"`
	Container ContentContainerPayload `json:"container"`
	Body      ContentBodyPayload      `json:"body"`
}

type CreatedPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Links Links  `json:"_links"`
}

type CreatedComment struct {
	ID    string `json:"id"`
	Links Links  `json:"_links"`
}

type ConfluenceWatcher struct {
	UserKey     string `json:"userKey"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type ContentWatchersResponse struct {
	Results []ConfluenceWatcher `json:"results"`
}

type SpaceOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type PageOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type SpaceListResponse struct {
	Results []SpaceResponse `json:"results"`
}

type ContentSearchResponse struct {
	Results []CreatedPage `json:"results"`
}

type CommentSearchResponse struct {
	Results []CommentResponse `json:"results"`
	Size    int               `json:"size"`
	Limit   int               `json:"limit"`
	Start   int               `json:"start"`
}

type DescendantCommentSearchResponse struct {
	Results []CommentResponse     `json:"results"`
	Comment CommentSearchResponse `json:"comment"`
}

type ConfluenceServerEvent struct {
	Comment *CommentResponse
	Page    *PageResponse
	Space   *SpaceResponse
	BaseURL string
}

func newServerClient(url string, httpClient *http.Client) Client {
	return &confluenceServerClient{
		URL:        url,
		HTTPClient: httpClient,
	}
}

func (csc *confluenceServerClient) GetSelf() (*types.ConfluenceUser, error) {
	confluenceServerUser := &ConfluenceServerUser{}
	if _, _, err := service.CallJSONWithURL(csc.URL, PathCurrentUser, http.MethodGet, nil, confluenceServerUser, csc.HTTPClient); err != nil {
		return nil, errors.Wrap(err, "Confluence GetSelf. Error getting the current user")
	}

	confluenceUser := &types.ConfluenceUser{
		AccountID:   confluenceServerUser.UserKey,
		Name:        confluenceServerUser.Username,
		DisplayName: confluenceServerUser.DisplayName,
	}

	return confluenceUser, nil
}

func (csc *confluenceServerClient) GetEventData(webhookPayload *serializer.ConfluenceServerWebhookPayload) (*ConfluenceServerEvent, error) {
	var confluenceServerEvent ConfluenceServerEvent
	var err error

	if strings.Contains(webhookPayload.Event, Comment) {
		confluenceServerEvent.Comment, err = csc.GetCommentData(webhookPayload)
		if err != nil {
			return nil, errors.Errorf("error getting comment data for the event. CommentID %d. Error: %v", webhookPayload.Comment.ID, err)
		}
	}

	if strings.Contains(webhookPayload.Event, Page) {
		confluenceServerEvent.Page, err = csc.GetPageData(int(webhookPayload.Page.ID))
		if err != nil {
			return nil, errors.Errorf("error getting page data for the event. PageID %d. Error: %v", webhookPayload.Page.ID, err)
		}
	}

	if strings.Contains(webhookPayload.Event, Space) {
		confluenceServerEvent.Space, err = csc.GetSpaceData(webhookPayload.Space.SpaceKey)
		if err != nil {
			return nil, errors.Errorf("error getting space data for the event. SpaceKey %s. Error: %v", webhookPayload.Space.SpaceKey, err)
		}
	}

	return &confluenceServerEvent, nil
}

func (csc *confluenceServerClient) GetCommentData(webhookPayload *serializer.ConfluenceServerWebhookPayload) (*CommentResponse, error) {
	commentID := strconv.FormatInt(webhookPayload.Comment.ID, 10)

	var lastErr error
	for _, delay := range []time.Duration{0, 250 * time.Millisecond, 750 * time.Millisecond} {
		if delay > 0 {
			time.Sleep(delay)
		}

		commentResponse := &CommentResponse{}
		commentPath := fmt.Sprintf("%s%s?status=any&expand=body.view,body.storage,container,space,history", PathContentData, commentID)
		if _, statusCode, err := service.CallJSONWithURL(csc.URL, commentPath, http.MethodGet, nil, commentResponse, csc.HTTPClient); err == nil {
			return commentResponse, nil
		} else {
			lastErr = err
			if statusCode == http.StatusNotFound {
				if webhookPayload.Page.ID != 0 {
					comment, descendantErr := csc.GetCommentDataFromPageDescendants(strconv.FormatInt(webhookPayload.Page.ID, 10), commentID)
					if descendantErr == nil {
						return comment, nil
					}
					lastErr = descendantErr
				} else {
					comment, searchErr := csc.SearchCommentDataByID(commentID)
					if searchErr == nil {
						return comment, nil
					}
					lastErr = searchErr
				}
			}
		}
	}

	return nil, lastErr
}

func (csc *confluenceServerClient) GetCommentDataFromPageDescendants(pageID, commentID string) (*CommentResponse, error) {
	const descendantPageSize = 200

	for start := 0; ; start += descendantPageSize {
		response := &DescendantCommentSearchResponse{}
		path := fmt.Sprintf("%s%s/descendant/comment?expand=body.view,body.storage,container,space,history&start=%d&limit=%d", PathContentData, pageID, start, descendantPageSize)
		if _, _, err := service.CallJSONWithURL(csc.URL, path, http.MethodGet, nil, response, csc.HTTPClient); err != nil {
			return nil, err
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

func (csc *confluenceServerClient) SearchCommentDataByID(commentID string) (*CommentResponse, error) {
	path := fmt.Sprintf("%s?cql=%s&expand=body.view,body.storage,container,space,history&limit=1", PathContentData+"search", url.QueryEscape("id="+commentID))
	response := &CommentSearchResponse{}
	if _, _, err := service.CallJSONWithURL(csc.URL, path, http.MethodGet, nil, response, csc.HTTPClient); err != nil {
		return nil, err
	}

	for i := range response.Results {
		if response.Results[i].ID == commentID {
			return &response.Results[i], nil
		}
	}

	return nil, errors.Errorf("comment %s not found via content search", commentID)
}

func getDescendantComments(response *DescendantCommentSearchResponse) []CommentResponse {
	if len(response.Results) > 0 {
		return response.Results
	}

	return response.Comment.Results
}

func (csc *confluenceServerClient) GetPageData(pageID int) (*PageResponse, error) {
	pageResponse := &PageResponse{}
	if _, _, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf("%s%s?status=any&expand=body.view,body.storage,container,space,history", PathContentData, strconv.Itoa(pageID)), http.MethodGet, nil, pageResponse, csc.HTTPClient); err != nil {
		return nil, err
	}

	return pageResponse, nil
}

func (csc *confluenceServerClient) GetSpaceData(spaceKey string) (*SpaceResponse, error) {
	spaceResponse := &SpaceResponse{}
	if _, _, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf("%s%s?status=any", PathSpaceData, spaceKey), http.MethodGet, nil, spaceResponse, csc.HTTPClient); err != nil {
		return nil, err
	}

	return spaceResponse, nil
}

type apiResponse struct {
	Results []struct {
		ID   int64  `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"results"`
	Size int `json:"size"`
}

func (csc *confluenceServerClient) GetSpaceKeyFromSpaceID(spaceID int64) (string, error) {
	start := 0

	for {
		path := fmt.Sprintf("%s?start=%d&limit=%d", PathSpaceData, start, pageSize)

		response := &apiResponse{}

		if _, _, err := service.CallJSONWithURL(csc.URL, path, http.MethodGet, nil, response, csc.HTTPClient); err != nil {
			return "", errors.Wrap(err, "Confluence GetSpaceKeyFromSpaceID")
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

	return "", fmt.Errorf("confluence GetSpaceKeyFromSpaceID: no space key found for the space ID")
}

type ConfluenceUser struct {
	DisplayName    string `json:"displayName"`
	Type           string `json:"type"`
	UserKey        string `json:"userKey"`
	Username       string `json:"username"`
	ProfilePicture struct {
		Path      string `json:"path"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		IsDefault bool   `json:"isDefault"`
	} `json:"profilePicture"`
	Expandable map[string]string `json:"_expandable"`
	Links      struct {
		Base    string `json:"base"`
		Context string `json:"context"`
		Self    string `json:"self"`
	} `json:"_links"`
}

func (csc *confluenceServerClient) GetUserFromUserKey(userKey string) (*ConfluenceUser, error) {
	var user ConfluenceUser

	if _, _, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf("%s?key=%s", PathUserData, userKey), http.MethodGet, nil, &user, csc.HTTPClient); err != nil {
		return nil, fmt.Errorf("error fetching user data: %w", err)
	}

	return &user, nil
}

func (csc *confluenceServerClient) CreatePage(in *CreatePageInput) (*CreatedPage, error) {
	payload := CreatePagePayload{
		Type:  "page",
		Title: strings.TrimSpace(in.Title),
		Space: ContentSpacePayload{Key: strings.TrimSpace(in.SpaceKey)},
		Body: ContentBodyPayload{
			Storage: ContentBodyStorage{
				Value:          in.Body,
				Representation: "storage",
			},
		},
	}

	if strings.TrimSpace(in.ParentPageID) != "" {
		payload.Ancestors = []ContentAncestorPayload{{ID: strings.TrimSpace(in.ParentPageID)}}
	}

	created := &CreatedPage{}
	if _, _, err := service.CallJSONWithURL(csc.URL, PathContentData, http.MethodPost, payload, created, csc.HTTPClient); err != nil {
		return nil, err
	}

	return created, nil
}

func (csc *confluenceServerClient) AddCommentToPage(in *CreateCommentInput) (*CreatedComment, error) {
	payload := CreateCommentPayload{
		Type: "comment",
		Container: ContentContainerPayload{
			ID:   strings.TrimSpace(in.PageID),
			Type: "page",
		},
		Body: ContentBodyPayload{
			Storage: ContentBodyStorage{
				Value:          in.Body,
				Representation: "storage",
			},
		},
	}

	created := &CreatedComment{}
	if _, _, err := service.CallJSONWithURL(csc.URL, PathContentData, http.MethodPost, payload, created, csc.HTTPClient); err != nil {
		return nil, err
	}

	return created, nil
}

func (csc *confluenceServerClient) GetContentWatchers(pageID string) ([]ConfluenceWatcher, error) {
	response := &ContentWatchersResponse{}
	if _, _, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf("%s%s/watchers", PathContentData, pageID), http.MethodGet, nil, response, csc.HTTPClient); err != nil {
		return nil, err
	}

	return response.Results, nil
}

func (csc *confluenceServerClient) GetAvailableSpaces() ([]SpaceOption, error) {
	response := &SpaceListResponse{}
	if _, _, err := service.CallJSONWithURL(csc.URL, fmt.Sprintf("%s?limit=100&type=GLOBAL", PathSpaceData), http.MethodGet, nil, response, csc.HTTPClient); err != nil {
		return nil, err
	}

	options := make([]SpaceOption, 0, len(response.Results))
	for _, result := range response.Results {
		label := result.Key
		if strings.TrimSpace(result.Name) != "" {
			label = fmt.Sprintf("%s (%s)", result.Name, result.Key)
		}
		options = append(options, SpaceOption{
			Value: result.Key,
			Label: label,
		})
	}

	return options, nil
}

func (csc *confluenceServerClient) SearchPages(spaceKey, query string) ([]PageOption, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return []PageOption{}, nil
	}

	cql := fmt.Sprintf("type=page AND space=\"%s\" AND title~\"%s*\"", strings.TrimSpace(spaceKey), escapeCQL(query))
	path := fmt.Sprintf("%s?cql=%s&limit=20", PathContentData+"search", url.QueryEscape(cql))

	response := &ContentSearchResponse{}
	if _, _, err := service.CallJSONWithURL(csc.URL, path, http.MethodGet, nil, response, csc.HTTPClient); err != nil {
		return nil, err
	}

	options := make([]PageOption, 0, len(response.Results))
	for _, result := range response.Results {
		options = append(options, PageOption{
			Value: result.ID,
			Label: result.Title,
		})
	}

	return options, nil
}

func formatMattermostPostForConfluence(postMessage, permalink string) string {
	message := strings.TrimSpace(postMessage)
	if message == "" {
		message = "_Original Mattermost message did not contain text._"
	}

	renderer := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
			extension.Strikethrough,
			extension.Table,
			extension.TaskList,
		),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithHardWraps(),
		),
	)

	var out bytes.Buffer
	if err := renderer.Convert([]byte(message), &out); err != nil {
		fallback := strings.Split(message, "\n")
		paragraphs := make([]string, 0, len(fallback))
		for _, line := range fallback {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			paragraphs = append(paragraphs, "<p>"+html.EscapeString(line)+"</p>")
		}
		if len(paragraphs) == 0 {
			paragraphs = append(paragraphs, "<p><em>Original Mattermost message did not contain text.</em></p>")
		}
		out.WriteString(strings.Join(paragraphs, ""))
	}

	out.WriteString(fmt.Sprintf("<p><a href=\"%s\">View original message in Mattermost</a></p>", html.EscapeString(permalink)))
	return out.String()
}

func escapeCQL(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
