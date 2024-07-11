package main

import (
	"embed"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

var (
	username = "admin"
	password = "password"
)

type Session struct {
	Username string
	Expiry   time.Time
}

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]Session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]Session),
	}
}

func (sm *SessionManager) CreateSession(username string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	sm.sessions[sessionID] = Session{
		Username: username,
		Expiry:   time.Now().Add(1 * time.Hour),
	}
	return sessionID
}

func (sm *SessionManager) GetSession(sessionID string) (Session, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists || session.Expiry.Before(time.Now()) {
		return Session{}, false
	}
	return session, true
}

func (sm *SessionManager) InvalidateSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, sessionID)
}

var sessionManager = NewSessionManager()

func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/login", serveLogin)
	http.HandleFunc("/admin", serveAdmin)
	http.HandleFunc("/static/", serveStatic)

	port := 8080
	fmt.Printf("Starting server at http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, _ := staticFiles.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		if r.FormValue("username") == username && r.FormValue("password") == password {
			sessionID := sessionManager.CreateSession(username)
			http.SetCookie(w, &http.Cookie{
				Name:    "session_id",
				Value:   sessionID,
				Expires: time.Now().Add(1 * time.Hour),
			})
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/login", http.StatusUnauthorized)
		return
	}
	data, _ := staticFiles.ReadFile("static/login.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func serveAdmin(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	session, valid := sessionManager.GetSession(cookie.Value)
	if !valid {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	tmpl, _ := template.ParseFS(staticFiles, "static/admin.html")
	tmpl.Execute(w, session)
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Path[len("/static/"):]
	data, err := staticFiles.ReadFile("static/" + filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ext := filepath.Ext(filePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Write(data)
}
