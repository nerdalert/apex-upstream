package oidcagent

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/pkg/ginsession"
	"github.com/nexodus-io/nexodus/pkg/oidcagent/models"
	"golang.org/x/oauth2"
)

const (
	TokenKey   = "token"
	IDTokenKey = "id_token"
)

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (o *OidcAgent) prepareContext(c *gin.Context) context.Context {
	if o.insecureTLS {
		parent := c.Request.Context()
		// #nosec: G402
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}
		return oidc.ClientContext(parent, client)
	}
	return c.Request.Context()
}

// LoginStart starts a login request
// @Summary      Start Web Login
// @Description  Starts a login request for the frontend application
// @Id 			 WebStart
// @Tags         Auth
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  models.LoginStartResponse "OK"
// @Failure      500  {string}  json "Internal Server Error"
// @Router       /web/login/start [post]
func (o *OidcAgent) LoginStart(c *gin.Context) {
	logger := o.logger
	state, err := randString(16)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	nonce, err := randString(16)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	logger = logger.With(
		"state", state,
		"nonce", nonce,
	)

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("state", state, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	c.SetCookie("nonce", nonce, int(time.Hour.Seconds()), "/", "", c.Request.URL.Scheme == "https", true)
	logger.Debug("set cookies")
	c.JSON(http.StatusOK, models.LoginStartResponse{
		AuthorizationRequestURL: o.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)),
	})
}

// LoginEnd completes the login request
// @Summary      End Web Login
// @Description  Handles the callback from the OAuth2 provider and completes the login process.
// @Id 			 WebEnd
// @Tags         Auth
// @Accepts		 json
// @Produce      json
// @Param        data  body models.LoginEndRequest  true "End Login"
// @Success      200 {object}  models.LoginEndResponse "Login successful"
// @Failure      400 {string}  json "Bad Request, usually due to state mismatch or other protocol errors"
// @Failure      401 {string}  json "Unauthorized, specifically when login is required"
// @Failure      500 {string}  json "Internal Server Error, including token validation or exchange issues"
// @Router       /web/login/end [post]
func (o *OidcAgent) LoginEnd(c *gin.Context) {
	var data models.LoginEndRequest
	err := c.BindJSON(&data)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	requestURL, err := url.Parse(data.RequestURL)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	logger := o.logger
	ctx := o.prepareContext(c)
	logger.Debug("handling login end request")

	values := requestURL.Query()
	code := values.Get("code")
	state := values.Get("state")
	queryErr := values.Get("error")

	failed := state != "" && queryErr != ""

	if failed {
		logger.Debug("login failed")
		var status int
		if queryErr == "login_required" {
			status = http.StatusUnauthorized
		} else {
			status = http.StatusBadRequest
		}
		c.AbortWithStatus(status)
		return
	}

	handleAuth := state != "" && code != ""

	loggedIn := false
	if handleAuth {
		logger.Debug("login success")
		originalState, err := c.Cookie("state")
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to access state cookie")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.SetCookie("state", "", -1, "/", "", c.Request.URL.Scheme == "https", true)
		if state != originalState {
			logger.With(
				"error", err,
			).Debug("state does not match")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		nonce, err := c.Cookie("nonce")
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to get nonce cookie")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.SetCookie("nonce", "", -1, "/", "", c.Request.URL.Scheme == "https", true)

		oauth2Token, err := o.oauthConfig.Exchange(ctx, code)
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to exchange token")
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			logger.With(
				"ok", ok,
			).Debug("unable to get id_token")
			_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("no id_token field in oauth2 token"))
			return
		}

		idToken, err := o.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			logger.With(
				"error", err,
			).Debug("unable to verify id_token")
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if idToken.Nonce != nonce {
			logger.Debug("nonce does not match")
			_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("nonce did not match"))
			return
		}

		session := ginsession.FromContext(c)
		tokenString, err := tokenToJSONString(oauth2Token)
		if err != nil {
			logger.Debug("can't convert token to string")
			_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("can't convert token to string"))
			return
		}
		session.Set(TokenKey, tokenString)
		session.Set(IDTokenKey, rawIDToken)
		if err := session.Save(); err != nil {
			logger.With("error", err,
				"id_token_size", len(rawIDToken)).Debug("can't save session storage")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Header("Authorization", fmt.Sprintf("Bearer %s", oauth2Token.AccessToken))

		logger.With("session_id", session.SessionID()).Debug("user is logged in")
		loggedIn = true
	} else {
		logger.Debug("checking if user is logged in")
		loggedIn = isLoggedIn(c)
	}

	session := ginsession.FromContext(c)
	logger.With("session_id", session.SessionID()).With("logged_in", loggedIn).Debug("complete")
	res := models.LoginEndResponse{
		Handled:  handleAuth,
		LoggedIn: loggedIn,
	}
	c.JSON(http.StatusOK, res)
}

// UserInfo gets information about the current user
// @Summary      Retrieve User Information
// @Description  Fetches the information of the currently logged-in user from the OAuth2 provider.
// @Id 			 UserInfo
// @Tags         Auth
// @Accepts		 json
// @Produce      json
// @Success      200 {object}  models.UserInfoResponse "Successfully retrieved user information"
// @Failure      401 {string}  json "Unauthorized, user not logged in or session expired"
// @Failure      500 {string}  json "Internal Server Error, during token validation or info retrieval"
// @Router       /web/user_info [get]
func (o *OidcAgent) UserInfo(c *gin.Context) {
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := JsonStringToToken(tokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	src := o.oauthConfig.TokenSource(ctx, token)

	info, err := o.provider.UserInfo(ctx, src)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var claims struct {
		Username   string `json:"preferred_username"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Picture    string `json:"picture"`
		UpdatedAt  int64  `json:"updated_at"`
	}

	err = info.Claims(&claims)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	o.logger.With("claims", claims).Debug("got claims from id_token")
	res := models.UserInfoResponse{
		Subject:           info.Subject,
		PreferredUsername: claims.Username,
		GivenName:         claims.GivenName,
		UpdatedAt:         int64(claims.UpdatedAt),
		FamilyName:        claims.FamilyName,
		Picture:           claims.Picture,
	}

	c.JSON(http.StatusOK, res)
}

// Claims gets the claims of the users access token
// @Summary      Claims
// @Description  Gets the claims of the users access token
// @Id 			 Claims
// @Tags         Auth
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  map[string]interface{} "OK"
// @Failure      401  {string}  json "Unauthorized"
// @Failure      500  {string}  json "Internal Server Error"
// @Router       /web/claims [get]
func (o *OidcAgent) Claims(c *gin.Context) {
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	idTokenRaw, ok := session.Get(IDTokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	idToken, err := o.verifier.Verify(ctx, idTokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var claims map[string]interface{}
	err = idToken.Claims(claims)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, claims)
}

// Refresh refreshes the access token and session ID
// @Summary      Refresh
// @Description  Refreshes the access token and updates the session ID
// @Id           Refresh
// @Tags         Auth
// @Accepts      json
// @Produce      json
// @Success      204
// @Failure      401  {string}  json "Unauthorized"
// @Failure      500  {string}  json "Internal Server Error"
// @Router       /web/refresh [get]
func (o *OidcAgent) Refresh(c *gin.Context) {
	session := ginsession.FromContext(c)

	ctx := o.prepareContext(c)

	// Existing token retrieval
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		o.logger.Debug("Token not found in session")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Token decoding
	token, err := JsonStringToToken(tokenRaw.(string))
	if err != nil {
		o.logger.Debug("Failed to decode token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Token refreshing
	src := o.oauthConfig.TokenSource(ctx, token)
	newToken, err := src.Token()
	if err != nil {
		o.logger.Debug("Failed to refresh token")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// New session ID generation
	newSID, err := generateNewSID()
	if err != nil {
		o.logger.Debug("Failed to generate a new session ID")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Convert new token to string
	tokenString, err := tokenToJSONString(newToken)
	if err != nil {
		o.logger.Debug("Can't convert refreshed token to string")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Updating session with new token and new session ID
	session.Set(TokenKey, tokenString)
	session.Set("sessionID", newSID)

	// Save session
	if err := session.Save(); err != nil {
		o.logger.Debug("Failed to save session")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Set the session ID as a cookie
	c.SetCookie("sessionID", newSID, 0, "/", "", c.Request.URL.Scheme == "https", true)
	c.Header("Authorization", fmt.Sprintf("Bearer %s", newToken.AccessToken))

	c.Status(http.StatusNoContent)
}

// Logout returns the URL to logout the current user
// @Summary      Logout
// @Description  Returns the URL to logout the current user
// @Id 			 Logout
// @Tags         Auth
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  models.LogoutResponse "OK"
// @Failure      401  {string}  json "Unauthorized"
// @Failure      500  {string}  json "Internal Server Error"
// @Router       /web/logout [post]
func (o *OidcAgent) Logout(c *gin.Context) {
	session := ginsession.FromContext(c)
	idToken, ok := session.Get(IDTokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	session.Delete(IDTokenKey)
	session.Delete(TokenKey)
	if err := session.Save(); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	logoutURL, err := o.LogoutURL(idToken.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, models.LogoutResponse{
		LogoutURL: logoutURL.String(),
	})
}

func (o *OidcAgent) CodeFlowProxy(c *gin.Context) {
	session := ginsession.FromContext(c)
	ctx := o.prepareContext(c)
	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token, err := JsonStringToToken(tokenRaw.(string))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// Use a static token source to avoid automatically
	// refreshing the token - this needs to be handled
	// by the frontend
	src := oauth2.StaticTokenSource(token)
	client := oauth2.NewClient(ctx, src)
	proxy := httputil.NewSingleHostReverseProxy(o.backend)

	// Use the client transport
	proxy.Transport = client.Transport
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = o.backend.Host
		req.URL.Scheme = o.backend.Scheme
		req.URL.Host = o.backend.Host
		req.URL.Path = c.Param("proxyPath")
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func isLoggedIn(c *gin.Context) bool {
	session := ginsession.FromContext(c)
	_, ok := session.Get(TokenKey)
	return ok
}

// DeviceStart starts a device login request
// @Summary      Start Login
// @Description  Starts a device login request
// @Id 			 DeviceStart
// @Tags         Auth
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  models.DeviceStartResponse
// @Router       /device/login/start [post]
func (o *OidcAgent) DeviceStart(c *gin.Context) {
	c.JSON(http.StatusOK, models.DeviceStartResponse{
		DeviceAuthURL: o.deviceAuthURL,
		Issuer:        o.oidcIssuer,
		ClientID:      o.clientID,
	})
}

func (o *OidcAgent) DeviceFlowProxy(c *gin.Context) {
	proxy := httputil.NewSingleHostReverseProxy(o.backend)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = o.backend.Host
		req.URL.Scheme = o.backend.Scheme
		req.URL.Host = o.backend.Host
		req.URL.Path = c.Param("proxyPath")
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func tokenToJSONString(t *oauth2.Token) (string, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func JsonStringToToken(s string) (*oauth2.Token, error) {
	var t oauth2.Token
	if err := json.Unmarshal([]byte(s), &t); err != nil {
		return nil, err
	}
	return &t, nil

}

// CheckAuth checks if the user is authenticated or not
// @Summary      Check Authenticated User
// @Description  Checks if the user is authenticated and returns appropriate status and message.
// @Id           CheckAuth
// @Tags         Auth
// @Accepts      json
// @Produce      json
// @Success      200  {object}  models.CheckAuthResponse "User is authenticated."
// @Failure      401  {object}  models.CheckAuthResponse "User is not authenticated."
// @Router       /web/check_auth [get]
func (o *OidcAgent) CheckAuth(c *gin.Context) {
	session := ginsession.FromContext(c)

	tokenRaw, ok := session.Get(TokenKey)
	if !ok {
		o.logger.Debug("Aborting with HTTP Status Unauthorized")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token, err := JsonStringToToken(tokenRaw.(string))
	if err != nil {
		o.logger.Debug("Failed to decode token %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if !token.Valid() {
		c.JSON(http.StatusUnauthorized, models.CheckAuthResponse{
			Status:  "failure",
			Message: "User is not authenticated.",
		})
		return
	}

	c.JSON(http.StatusOK, models.CheckAuthResponse{
		Status:  "success",
		Message: "User is authenticated.",
	})
}

// generateNewSID Generates a new Session ID using crypto/rand
func generateNewSID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
