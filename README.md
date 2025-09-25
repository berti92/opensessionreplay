# Session Recorder Projekt

Ein vollständiges Session-Recording System mit rrweb, das Benutzerinteraktionen auf Webseiten aufzeichnet und in einem Admin-Backend zur Wiedergabe bereitstellt.

## Features

- **Session-Aufzeichnung**: Automatische Aufzeichnung aller Benutzerinteraktionen mit rrweb
- **Go Backend**: Leichtgewichtiger Server in einer einzigen Datei
- **SQLite Datenbank**: Lokale Speicherung aller Session-Daten
- **Admin Interface**: Übersichtliche Verwaltung mit Paginierung
- **Session Replay**: Vollständige Wiedergabe der aufgezeichneten Sessions
- **Browser-kompatibel**: Funktioniert in allen modernen Browsern

## Installation

### 1. Go Dependencies installieren

```bash
go mod tidy
```

### 2. Server starten

```bash
go run main.go
```

Der Server läuft dann auf `http://localhost:8080`

## Nutzung

### 1. Session Recorder in Webseite einbinden

Fügen Sie diese Skripte in Ihre HTML-Seite ein:

```html
<!-- rrweb Library -->
<script src="https://cdn.jsdelivr.net/npm/rrweb@latest/dist/rrweb.min.js"></script>

<!-- Session Recorder Script -->
<script src="http://localhost:8080/recorder.js"></script>
```

**✅ Das war's! Keine weiteren Skripte oder Abhängigkeiten erforderlich.**

### 2. Demo-Seite testen

Öffnen Sie `demo.html` in Ihrem Browser oder besuchen Sie eine Webseite mit integriertem Recorder.

### 3. Admin-Interface nutzen

Besuchen Sie `http://localhost:8080` um alle aufgezeichneten Sessions zu sehen.

## Projektstruktur

```
opensessionreplay/
├── main.go           # Go Backend Server (SQLite, API, Admin UI)
├── recorder.js       # Client-seitiges Recording Script
├── demo.html         # Demo-Webseite zum Testen
├── go.mod           # Go Module Definition
├── package.json     # Node.js Dependencies (rrweb)
└── README.md        # Diese Datei
```

## API Endpoints

- `GET /` - Admin Interface
- `POST /api/sessions/metadata` - Session Metadaten speichern
- `POST /api/sessions/events` - Session Events speichern
- `GET /api/sessions` - Sessions abrufen (mit Paginierung)
- `GET /session/{id}` - Session Replay Seite
- `GET /recorder.js` - Recorder Script ausliefern

## Konfiguration

### Backend (main.go)

- **Port**: Standard 8080 (änderbar in `main()` Funktion)
- **Datenbank**: `sessions.db` (SQLite)
- **CORS**: Aktiviert für alle Origins

### Client (recorder.js)

```javascript
const CONFIG = {
    apiEndpoint: 'http://localhost:8080/api/sessions',
    batchSize: 50,        // Events pro Batch
    batchTimeout: 5000,   // Batch-Timeout in ms
};
```

## Session Datenbank Schema

```sql
CREATE TABLE sessions (
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
```

## Erweiterte Features

### CSS Klassen für Recording-Kontrolle

- `.sr-block` - Elemente werden komplett blockiert/ignoriert
- `.sr-ignore` - Elemente werden ignoriert aber DOM-Struktur bleibt

### Session Replay Features

- ✅ Vollständige DOM-Wiedergabe
- ✅ CSS-Animationen und Transitions
- ✅ Formular-Interaktionen
- ✅ Scroll-Verhalten
- ✅ Maus-Bewegungen und Klicks
- ✅ Keyboard-Eingaben
- ✅ Canvas-Aufzeichnung (optional)

## Sicherheitshinweise

⚠️ **Wichtig**: Dieses System ist für Entwicklungs- und Testzwecke konzipiert.

Für Produktionsumgebungen beachten Sie:

- CORS-Konfiguration einschränken
- Authentifizierung für Admin-Interface hinzufügen
- HTTPS verwenden
- Datenschutz-Richtlinien beachten
- Session-Daten regelmäßig bereinigen

## Dependencies

### Go
- `github.com/mattn/go-sqlite3` - SQLite Driver

### JavaScript
- `rrweb` - Session Recording Library

## Browser Support

- Chrome/Chromium 60+
- Firefox 55+
- Safari 11+
- Edge 79+

## Troubleshooting

### Server startet nicht
```bash
# CGO für SQLite aktivieren
CGO_ENABLED=1 go run main.go
```

### Sessions werden nicht aufgezeichnet
1. Browser Konsole auf Fehler prüfen
2. Netzwerk-Tab überprüfen (API Calls)
3. CORS-Probleme ausschließen

### Replay funktioniert nicht
1. Events in Datenbank vorhanden prüfen
2. Browser-Kompatibilität überprüfen
3. JavaScript-Konsole auf Fehler prüfen

## Lizenz

MIT License - Siehe Projektlizenz für Details.