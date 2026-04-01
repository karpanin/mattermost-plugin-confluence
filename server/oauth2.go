package main

import "net/http"

const (
	routeUserConnect        = "/oauth2/connect"
	routeUserComplete       = "/oauth2/complete.html"
	routeUserConnectionInfo = "/user-connection-info"
	routeDebugOAuthToken    = "/debug/oauth-token"
)

var userConnect = &Endpoint{
	Path:            routeUserConnect,
	Method:          http.MethodGet,
	Execute:         httpOAuth2Connect,
	IsAuthenticated: true,
}

var userConnectComplete = &Endpoint{
	Path:            routeUserComplete,
	Method:          http.MethodGet,
	Execute:         httpOAuth2Complete,
	IsAuthenticated: true,
}

var userConnectionInfo = &Endpoint{
	Path:            routeUserConnectionInfo,
	Method:          http.MethodGet,
	Execute:         httpGetUserInfo,
	IsAuthenticated: true,
}

var debugOAuthToken = &Endpoint{
	Path:            routeDebugOAuthToken,
	Method:          http.MethodGet,
	Execute:         httpGetDebugOAuthToken,
	IsAuthenticated: true,
}
