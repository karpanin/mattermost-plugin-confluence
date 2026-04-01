package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
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

func handleGetCreatePageSpaces(w http.ResponseWriter, r *http.Request, p *Plugin) {
	client, err := getCreatePageClient(p, r)
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
	_ = json.NewEncoder(w).Encode(spaces)
}

func handleSearchCreatePageParents(w http.ResponseWriter, r *http.Request, p *Plugin) {
	spaceKey := strings.TrimSpace(r.URL.Query().Get("space_key"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if spaceKey == "" {
		http.Error(w, "space_key is required.", http.StatusBadRequest)
		return
	}

	client, err := getCreatePageClient(p, r)
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

func getCreatePageClient(p *Plugin, r *http.Request) (Client, error) {
	userID := r.Header.Get(config.HeaderMattermostUserID)
	pluginConfig := config.GetConfig()
	connection, err := store.LoadConnection(pluginConfig.ConfluenceURL, userID)
	if err != nil {
		return nil, err
	}
	if connection.ConfluenceAccountID() == "" {
		return nil, errors.New("User not connected. Please use `/confluence connect`.")
	}

	return p.GetServerClient(pluginConfig.ConfluenceURL, connection)
}
