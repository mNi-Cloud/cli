package oidc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/gommon/log"
	"github.com/mNi-Cloud/cli/internal/pkg/pkce"
	"golang.org/x/oauth2"
)

type (
	Server struct {
		Res  chan Response
		stop chan bool
		flow oidcFlow
	}

	Response struct {
		IdToken *string
		Error   *error
	}
)

func NewServer(flow oidcFlow) *Server {
	return &Server{
		Res:  make(chan Response),
		stop: make(chan bool),
		flow: flow,
	}
}

func (s *Server) Serve() error {
	http.HandleFunc("/", s.redirectHandler)
	http.HandleFunc("/auth/callback", s.callbackHandler)

	listener, err := net.Listen("tcp", "localhost:51850")
	if err != nil {
		return err
	}
	go func() {
		err := http.Serve(listener, nil)
		s.Res <- Response{Error: &err}
	}()

	go func() {
		<-s.stop
		listener.Close()
	}()

	fmt.Println("Waiting for the browser to be opened...")
	err = openbrowser("http://localhost:51850/")
	if err != nil {
		log.Debug("Failed to open browser: " + err.Error())
		fmt.Println("Open http://localhost:51850/ in your browser!\nWaiting for the callback...")
		return nil
	}

	return nil
}

func openbrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

func (s *Server) redirectHandler(resp http.ResponseWriter, r *http.Request) {
	pkceCode, err := pkce.Generate()
	s.addPkceCookie(pkceCode, resp)
	state := s.addStateCookie(resp)

	if err != nil {
		http.Error(resp, "Failed to generate pkce challenge", http.StatusInternalServerError)
		return
	}

	http.Redirect(resp, r, s.flow.oauth2Config.AuthCodeURL(state, pkceCode.Challenge(), pkceCode.Method()), http.StatusFound)
}

func (s *Server) callbackHandler(resp http.ResponseWriter, req *http.Request) {
	err := s.checkStateAndExpireCookie(req, resp)

	if err != nil {
		s.redirectHandler(resp, req)
		return
	}

	tokenResponse, err := s.exchangeCode(req)

	if err != nil {
		http.Error(resp, "Failed to exchange code", http.StatusBadRequest)
		return
	}

	idToken, ok := tokenResponse.Extra("id_token").(string)

	if !ok {
		http.Error(resp, "Failed to extract id_token", http.StatusBadRequest)
	}

	// send 200 to resp
	resp.WriteHeader(200)
	resp.Write([]byte("Success"))

	s.stop <- true
	s.Res <- Response{IdToken: &idToken}
}

func (s *Server) addStateCookie(resp http.ResponseWriter) string {
	expire := time.Now().Add(1 * time.Minute)
	value := uuid.New().String()

	cookie := http.Cookie{
		Name:     "p_state",
		Value:    value,
		Expires:  expire,
		HttpOnly: true,
	}

	http.SetCookie(resp, &cookie)

	return value
}

func (s *Server) addPkceCookie(code pkce.Code, resp http.ResponseWriter) {
	expire := time.Now().Add(1 * time.Minute)
	value := string(code)

	cookie := http.Cookie{
		Name:     "p_pkce",
		Value:    value,
		Expires:  expire,
		HttpOnly: true,
	}

	http.SetCookie(resp, &cookie)
}

func (s *Server) expireCookie(name string, resp http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "p_state",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
	}

	http.SetCookie(resp, cookie)
}

func (s *Server) checkStateAndExpireCookie(req *http.Request, resp http.ResponseWriter) error {
	state, err := req.Cookie("p_state")

	s.expireCookie("p_state", resp)

	if err != nil {
		return errors.New("state cookie not set")
	}

	if req.URL.Query().Get("state") != state.Value {
		return errors.New("invalid state")
	}

	return nil
}

func (s *Server) exchangeCode(req *http.Request) (*oauth2.Token, error) {
	httpClient := &http.Client{Timeout: 2 * time.Second}
	ctx := context.WithValue(req.Context(), oauth2.HTTPClient, httpClient)
	pkceCookie, err := req.Cookie("p_pkce")
	pkceCode := pkce.Code(pkceCookie.Value)

	tokenResponse, err := s.flow.oauth2Config.Exchange(ctx, req.URL.Query().Get("code"), pkceCode.Verifier())

	if err != nil {
		return nil, err
	}

	return tokenResponse, nil
}
