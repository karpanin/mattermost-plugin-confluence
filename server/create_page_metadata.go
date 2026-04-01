package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

var getCreatePageSpaces = &Endpoint{
	Path:            "/page/spaces",
	Method:          http.MethodGet,
	Execute:         handleGetCreatePageSpaces,
	IsAuthenticated: true,
}

var searchCreatePageParents = &Endpoint{
	Path:            "/page/parents",
	Method:          http.MethodGet,
	Execute:         handleSearchCreatePageParents,
	IsAuthenticated: true,
}

type CreatePageSpacesResponse struct {
	Spaces               []SpaceOption `json:"spaces"`
	LastSelectedSpaceKey string        `json:"lastSelectedSpaceKey"`
}

func handleGetCreatePageSpaces(w http.ResponseWriter, r *http.Request, p *Plugin) {
	client, connection, err := getCreatePageClient(p, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	spaces, err := client.GetAvailableSpaces()
	if err != nil {
		p.client.Log.Error("Failed to load Confluence spaces", "error", err.Error())
		http.Error(w, "Failed to load Confluence spaces: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	lastSelectedSpaceKey := ""
	if connection != nil && connection.Settings != nil {
		lastSelectedSpaceKey = connection.Settings.LastSelectedSpaceKey
	}
	_ = json.NewEncoder(w).Encode(CreatePageSpacesResponse{
		Spaces:               spaces,
		LastSelectedSpaceKey: lastSelectedSpaceKey,
	})
}

func handleSearchCreatePageParents(w http.ResponseWriter, r *http.Request, p *Plugin) {
	spaceKey := strings.TrimSpace(r.URL.Query().Get("space_key"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if spaceKey == "" {
		http.Error(w, "space_key is required.", http.StatusBadRequest)
		return
	}

	client, _, err := getCreatePageClient(p, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	pages, err := client.SearchPages(spaceKey, query)
	if err != nil {
		p.client.Log.Error("Failed to search Confluence pages", "space_key", spaceKey, "query", query, "error", err.Error())
		http.Error(w, "Failed to search Confluence pages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pages)
}

func getCreatePageClient(p *Plugin, r *http.Request) (Client, *types.Connection, error) {
	userID := r.Header.Get(config.HeaderMattermostUserID)
	pluginConfig := config.GetConfig()
	connection, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
	if err != nil {
		return nil, nil, err
	}
	if connection.ConfluenceAccountID() == "" {
		return nil, nil, errors.New("User not connected. Please use `/confluence connect`.")
	}

	client, err := p.GetServerClient(pluginConfig.ConfluenceURL, connection)
	if err != nil {
		return nil, nil, err
	}
	return client, connection, nil
}

func saveLastSelectedSpaceForUser(userID, spaceKey string) error {
	spaceKey = strings.TrimSpace(spaceKey)
	if userID == "" || spaceKey == "" {
		return nil
	}

	pluginConfig := config.GetConfig()
	connection, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
	if err != nil {
		return err
	}

	ensureConnectionSettings(connection)
	connection.Settings.LastSelectedSpaceKey = spaceKey

	return store.StoreConnection(pluginConfig.ConfluenceURL, userID, connection)
}
