package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Session represents a recorded session
type Session struct {
	ID        int       `json:"id"`
	SessionID string    `json:"session_id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	UserAgent string    `json:"user_agent"`
	Events    string    `json:"events"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Viewport  string    `json:"viewport"`
}

// SessionMetadata represents session metadata
type SessionMetadata struct {
	SessionID string `json:"sessionId"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	UserAgent string `json:"userAgent"`
	Timestamp string `json:"timestamp"`
	Viewport  struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"viewport"`
}

// EventBatch represents a batch of rrweb events
type EventBatch struct {
	SessionID string        `json:"sessionId"`
	Events    []interface{} `json:"events"`
	Timestamp string        `json:"timestamp"`
}

// Server holds the database connection and handlers
type Server struct {
	db         *sql.DB
	username   string
	password   string
	rrWebJs    string
	recorderJs string
	proxyURL   string
}

func main() {
	server := &Server{}

	// Initialize database
	if err := server.initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer server.db.Close()

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get auth credentials from environment variables
	server.username = os.Getenv("BASIC_AUTH_USER")
	server.password = os.Getenv("BASIC_AUTH_PASS")
	server.rrWebJs = os.Getenv("RRWEB_JS_NAME")
	server.recorderJs = os.Getenv("RECORDER_JS_NAME")
	server.proxyURL = os.Getenv("PROXY_URL")

	if server.username == "" {
		server.username = "admin"
	}
	if server.password == "" {
		log.Println("Warning: BASIC_AUTH_PASS not set, using default password")
		server.password = "admin"
	}
	if server.rrWebJs == "" {
		log.Println("Warning: RRWEB_JS_NAME not set, using default rrweb.min.js")
		server.rrWebJs = "rrweb.min.js"
	}
	if server.recorderJs == "" {
		log.Println("Warning: RECORDER_JS_NAME not set, using default recorder.js")
		server.recorderJs = "recorder.js"
	}
	if server.proxyURL == "" {
		server.proxyURL = fmt.Sprintf("http://localhost:%s", port)
	}

	// Routes
	// Admin routes with BasicAuth
	http.HandleFunc("/", server.basicAuth(server.adminHandler))
	http.HandleFunc("/api/sessions", server.basicAuth(server.corsMiddleware(server.getSessionsHandler)))
	http.HandleFunc("/session/", server.basicAuth(server.viewSessionHandler))

	// Public API routes (for recording)
	http.HandleFunc("/api/sessions/metadata", server.corsMiddleware(server.sessionMetadataHandler))
	http.HandleFunc("/api/sessions/events", server.corsMiddleware(server.sessionEventsHandler))

	// Static files
	http.HandleFunc("/"+server.recorderJs, server.serveRecorderJS)
	http.HandleFunc("/"+server.rrWebJs, server.serveRrwebJS)
	http.HandleFunc("/rrweb-player.js", server.serveRrwebPlayerJS)
	http.HandleFunc("/rrweb-player.css", server.serveRrwebPlayerCSS)

	fmt.Printf("Server starting on :%s\n", port)
	fmt.Printf("Admin interface: http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func (s *Server) initDB() error {
	var err error
	// Use data directory if it exists, otherwise current directory
	dbPath := "./sessions.db"
	if _, err := os.Stat("./data"); err == nil {
		dbPath = "./data/sessions.db"
	}

	s.db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create sessions table
	query := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT UNIQUE NOT NULL,
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		user_agent TEXT NOT NULL,
		events TEXT DEFAULT '',
		viewport TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_session_id ON sessions(session_id);
	CREATE INDEX IF NOT EXISTS idx_created_at ON sessions(created_at DESC);
	`

	_, err = s.db.Exec(query)
	return err
}

func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func (s *Server) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(s.username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(s.password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) sessionMetadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metadata SessionMetadata
	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get client IP
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}
	// Remove port from IP if present
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		// Check if this is IPv6 (has multiple colons) or IPv4:port
		if strings.Count(clientIP, ":") == 1 {
			// IPv4:port format
			clientIP = clientIP[:idx]
		} else if strings.HasPrefix(clientIP, "[") {
			// IPv6 format like [::1]:port
			if endIdx := strings.LastIndex(clientIP, "]:"); endIdx != -1 {
				clientIP = clientIP[1:endIdx]
			}
		}
	}

	// Extract domain from URL
	parsedURL, err := url.Parse(metadata.URL)
	var domain string
	if err == nil {
		domain = parsedURL.Host
	} else {
		domain = "unknown"
	}

	// Generate deterministic session ID from date + domain + IP + user agent
	today := time.Now().Format("2006-01-02")
	sessionID := generateSessionID(today, domain, clientIP, metadata.UserAgent)

	viewport, _ := json.Marshal(metadata.Viewport)
	now := time.Now()

	// Check if session already exists
	var existingEvents string
	err = s.db.QueryRow("SELECT events FROM sessions WHERE session_id = ?", sessionID).Scan(&existingEvents)

	if err == sql.ErrNoRows {
		// Create new session
		query := `
		INSERT INTO sessions (session_id, url, title, user_agent, viewport, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err = s.db.Exec(query, sessionID, metadata.URL, metadata.Title,
			metadata.UserAgent, string(viewport), now, now)
		if err != nil {
			log.Printf("Error creating session: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else if err == nil {
		// Update existing session (merge)
		query := `
		UPDATE sessions
		SET url = ?, title = ?, user_agent = ?, viewport = ?, updated_at = ?
		WHERE session_id = ?
		`
		_, err = s.db.Exec(query, metadata.URL, metadata.Title,
			metadata.UserAgent, string(viewport), now, sessionID)
		if err != nil {
			log.Printf("Error updating session: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		log.Printf("Error checking session: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return the actual session ID that will be used
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"sessionId": sessionID,
	})
}

// generateSessionID creates a deterministic session ID from date, domain, IP and user agent
func generateSessionID(date, domain, ip, userAgent string) string {
	hasher := sha256.New()
	hasher.Write([]byte(date + "|" + domain + "|" + ip + "|" + userAgent))
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

func (s *Server) sessionEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var batch EventBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Since client doesn't know our generated session ID, we need to find it
	// We'll use the most recent session that matches today's date
	// Get client IP
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}
	// Remove port from IP if present
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		// Check if this is IPv6 (has multiple colons) or IPv4:port
		if strings.Count(clientIP, ":") == 1 {
			// IPv4:port format
			clientIP = clientIP[:idx]
		} else if strings.HasPrefix(clientIP, "[") {
			// IPv6 format like [::1]:port
			if endIdx := strings.LastIndex(clientIP, "]:"); endIdx != -1 {
				clientIP = clientIP[1:endIdx]
			}
		}
	}

	// Find today's session for this IP and user agent
	// We need the user agent from the request header
	userAgent := r.Header.Get("User-Agent")

	// Try to find the session by matching date and user agent in the database
	query := `
	SELECT session_id, events
	FROM sessions
	WHERE date(created_at) = date('now', 'localtime')
	AND user_agent = ?
	ORDER BY updated_at DESC
	LIMIT 1
	`

	var sessionID string
	var existingEvents string
	err := s.db.QueryRow(query, userAgent).Scan(&sessionID, &existingEvents)

	if err != nil {
		log.Printf("Error finding session for events: %v (client IP: %s)", err, clientIP)
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Append new events
	var allEvents []interface{}
	if existingEvents != "" {
		if err := json.Unmarshal([]byte(existingEvents), &allEvents); err != nil {
			log.Printf("Error unmarshaling existing events: %v", err)
		}
	}

	allEvents = append(allEvents, batch.Events...)
	eventsJSON, _ := json.Marshal(allEvents)

	// Update session with new events
	updateQuery := `UPDATE sessions SET events = ?, updated_at = ? WHERE session_id = ?`
	_, err = s.db.Exec(updateQuery, string(eventsJSON), time.Now(), sessionID)
	if err != nil {
		log.Printf("Error updating session events: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "sessionId": sessionID})
}

func (s *Server) getSessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit := 20
	offset := (page - 1) * limit

	// Get total count
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE events != ''").Scan(&total)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get sessions
	query := `
	SELECT id, session_id, url, title, user_agent, created_at, updated_at, viewport
	FROM sessions 
	WHERE events != ''
	ORDER BY updated_at DESC 
	LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		err := rows.Scan(&session.ID, &session.SessionID, &session.URL, &session.Title,
			&session.UserAgent, &session.CreatedAt, &session.UpdatedAt, &session.Viewport)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	response := map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"limit":    limit,
		"pages":    (total + limit - 1) / limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) adminHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="de">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Session Recorder Admin</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007cba; padding-bottom: 10px; }
        .sessions-table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        .sessions-table th, .sessions-table td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        .sessions-table th { background-color: #007cba; color: white; }
        .sessions-table tr:hover { background-color: #f5f5f5; }
        .session-link { color: #007cba; text-decoration: none; font-weight: bold; }
        .session-link:hover { text-decoration: underline; }
        .pagination { margin: 20px 0; text-align: center; }
        .pagination a, .pagination span { margin: 0 5px; padding: 8px 12px; text-decoration: none; border: 1px solid #ddd; color: #007cba; }
        .pagination .current { background-color: #007cba; color: white; }
        .user-agent { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 0.9em; color: #666; }
        .loading { text-align: center; padding: 40px; color: #666; }
        .error { color: #d32f2f; text-align: center; padding: 20px; }
        .stats { background: #e3f2fd; padding: 15px; border-radius: 4px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎥 Session Recorder Admin</h1>
        <div id="stats" class="stats"></div>
        <div id="content" class="loading">Lade Sessions...</div>
    </div>

    <script>
        let currentPage = 1;
        
        async function loadSessions(page = 1) {
            try {
                document.getElementById('content').innerHTML = '<div class="loading">Lade Sessions...</div>';
                
                const response = await fetch('/api/sessions?page=' + page);
                const data = await response.json();
                
                currentPage = page;
                renderSessions(data);
                renderStats(data);
            } catch (error) {
			    console.log('Error loading sessions:', error);
                document.getElementById('content').innerHTML = '<div class="error">No data to display</div>';
            }
        }
        
        function renderStats(data) {
            const stats = document.getElementById('stats');
            stats.innerHTML = '📊 <strong>' + data.total + '</strong> total sessions | page <strong>' + data.page + '</strong> from <strong>' + data.pages + '</strong>';
        }
        
        function renderSessions(data) {
            const content = document.getElementById('content');
            
            if (data.sessions.length === 0) {
                content.innerHTML = '<div class="error">No sessions found.</div>';
                return;
            }
            
            let html = '<table class="sessions-table"><thead><tr>';
            html += '<th>Session ID</th>';
            html += '<th>URL</th>';
            html += '<th>Title</th>';
            html += '<th>Date/Time</th>';
            html += '<th>Browser Agent</th>';
            html += '<th>Action</th>';
            html += '</tr></thead><tbody>';
            
            data.sessions.forEach(session => {
                const date = new Date(session.created_at).toLocaleString('de-DE');
                html += '<tr>';
                html += '<td>' + session.session_id.substring(0, 20) + '...</td>';
                html += '<td><a href="' + session.url + '" target="_blank">' + (session.url.length > 50 ? session.url.substring(0, 50) + '...' : session.url) + '</a></td>';
                html += '<td>' + session.title + '</td>';
                html += '<td>' + date + '</td>';
                html += '<td class="user-agent">' + session.user_agent + '</td>';
                html += '<td><a href="/session/' + session.session_id + '" class="session-link" target="_blank">📽️ View</a></td>';
                html += '</tr>';
            });
            
            html += '</tbody></table>';
            
            // Pagination
            html += '<div class="pagination">';
            if (data.page > 1) {
                html += '<a href="#" onclick="loadSessions(' + (data.page - 1) + ')">← Vorherige</a>';
            }
            
            for (let i = Math.max(1, data.page - 2); i <= Math.min(data.pages, data.page + 2); i++) {
                if (i === data.page) {
                    html += '<span class="current">' + i + '</span>';
                } else {
                    html += '<a href="#" onclick="loadSessions(' + i + ')">' + i + '</a>';
                }
            }
            
            if (data.page < data.pages) {
                html += '<a href="#" onclick="loadSessions(' + (data.page + 1) + ')">Nächste →</a>';
            }
            html += '</div>';
            
            content.innerHTML = html;
        }
        
        // Load sessions on page load
        loadSessions();
        
        // Auto-refresh every 30 seconds
        setInterval(() => loadSessions(currentPage), 30000);
    </script>
</body>
</html>
`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func (s *Server) viewSessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/session/")

	var session Session
	query := `SELECT session_id, url, title, user_agent, events, created_at, viewport FROM sessions WHERE session_id = ?`
	err := s.db.QueryRow(query, sessionID).Scan(
		&session.SessionID, &session.URL, &session.Title,
		&session.UserAgent, &session.Events, &session.CreatedAt, &session.Viewport)

	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Validate that events is valid JSON
	if session.Events == "" {
		session.Events = "[]"
	}

	tmpl := `
<!DOCTYPE html>
<html lang="de">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Session Replay: {{.Title}}</title>
    <link rel="stylesheet" href="/rrweb-player.css">
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            background: #1a1a1a;
            color: white;
        }
        .header {
            background: #333;
            padding: 15px;
            border-bottom: 2px solid #007cba;
        }
        .header h1 {
            margin: 0;
            color: #007cba;
        }
        .session-info {
            background: #222;
            padding: 10px 15px;
            font-size: 14px;
        }
        .session-info span {
            margin-right: 20px;
        }
        #player {
            width: 100%;
            height: calc(100vh - 100px);
        }
        .error {
            color: #f44336;
            text-align: center;
            padding: 20px;
        }
    </style>
    <script src="/{{.RrWebJs}}"></script>
    <script src="/rrweb-player.js"></script>
</head>
<body>
    <div class="header">
        <h1>🎥 Session Replay</h1>
    </div>

    <div class="session-info">
        <span><strong>URL:</strong> <a href="{{.URL}}" target="_blank" style="color: #007cba;">{{.URL}}</a></span>
        <span><strong>Title:</strong> {{.Title}}</span>
        <span><strong>Date:</strong> {{.CreatedAt.Format "02.01.2006 15:04:05"}}</span>
    </div>

    <div id="player"></div>

    <script>
        let events = [];
        try {
            // Events are already a JSON string from the database
            events = {{rawJS .Events}};
        } catch (error) {
            console.error('Error parsing events:', error);
            events = [];
        }

        // Initialize the rrweb player when page loads
        document.addEventListener('DOMContentLoaded', function() {
            if (!events || !Array.isArray(events) || events.length === 0) {
                document.getElementById('player').innerHTML = '<div class="error">Keine Aufzeichnungsdaten verfügbar.</div>';
                return;
            }

            try {
                // Create the rrweb player with built-in controls
                new rrwebPlayer({
                    target: document.getElementById('player'),
                    props: {
                        events: events,
                        width: window.innerWidth,
                        height: window.innerHeight - 100,
                        autoPlay: false,
                        showController: true,
                        skipInactive: true
                    }
                });

                console.log('rrweb Player initialized with', events.length, 'events');
            } catch (error) {
                console.error('Error initializing rrweb player:', error);
                document.getElementById('player').innerHTML = '<div class="error">Fehler beim Laden der Aufzeichnung: ' + error.message + '</div>';
            }
        });
    </script>
</body>
</html>
`

	// Create a template with custom function to output raw JS
	funcMap := template.FuncMap{
		"rawJS": func(s string) template.JS {
			return template.JS(s)
		},
	}

	t, err := template.New("session").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Session
		RrWebJs string
	}{
		Session: session,
		RrWebJs: s.rrWebJs,
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, data)
}

func (s *Server) serveRecorderJS(w http.ResponseWriter, r *http.Request) {
	// Read the recorder.js file
	content, err := os.ReadFile("./recorder.js")
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Replace the placeholder with actual proxy URL
	modifiedContent := strings.Replace(
		string(content),
		"http://localhost:8080",
		s.proxyURL,
		1,
	)

	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(modifiedContent))
}

func (s *Server) serveRrwebJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./node_modules/rrweb/dist/rrweb.min.js")
}

func (s *Server) serveRrwebPlayerJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./node_modules/rrweb-player/dist/index.js")
}

func (s *Server) serveRrwebPlayerCSS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./node_modules/rrweb-player/dist/style.css")
}
