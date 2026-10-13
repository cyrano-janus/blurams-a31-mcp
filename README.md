# blurams-a31-mcp

Experimenteller Go-MCP-Server für Blurams-Schwenk-/Neigekameras, zunächst die A31.
Verwendet das **offizielle [MCP-Go-SDK](https://github.com/modelcontextprotocol/go-sdk)**,
Version v1.8.0, mit typisierten Werkzeugen, stdio und stateless Streamable HTTP.

## Roadmap

Die geplante Weiterentwicklung mit Render, Descope und ChatGPT steht in
[ROADMAP.md](ROADMAP.md). Nächster Schritt ist die echte A31-Verbindungsprüfung;
OAuth und regelmäßige Steuerung folgen nach erfolgreicher Abnahme.

## Status und Grenzen

Die reale Anmeldung mit einem Blurams-Konto und die Kompatibilität mit einer A31
sind noch nicht bestätigt. Die Blurams-Anbindung basiert auf einem
undokumentierten Cloud-Protokoll. Insbesondere der kompatible Anmeldeweg über
Ease Life kann für ein Blurams-Konto fehlschlagen. Tests verwenden simulierte
Herstellerantworten und einen lokalen WebSocket-Server. Dieses Projekt ist
keine offizielle Blurams-Software.

Der HTTP-Zugang verwendet einen statischen Bearer-Token. Es gibt **keinen
GitHub-OAuth-Anmeldefluss**. Für einen Client, der OAuth verlangt, muss zuerst
eine passende OAuth-Anbindung ergänzt werden. Livestream und Bildanalyse sind
nicht implementiert. Es wurden keine wiederkehrenden Automationen aktiviert.

## Werkzeuge

| Werkzeug | Funktion |
| --- | --- |
| list_cameras | Kameras, Modell, Online-Status und PTZ-Fähigkeiten lesen |
| camera_status | Status einer ausgewählten Kamera lesen |
| move_camera | home, left, right, up oder down; ein bis fünf Schritte |

BLURAMS_CAMERA_ID beschränkt Status und Bewegungen auf eine feste Kamera.
Eine andere ID wird abgewiesen. Ohne feste ID muss ein Konto mit mehreren
Kameras eine ID pro Aufruf angeben. Die Liste enthält alle erkannten Kameras.

## Lokal starten

Voraussetzung: Go 1.25 oder neuer. Build-Compiler: Go 1.27.2.

```sh
go mod download
go build -trimpath -o bin/blurams-mcp ./cmd/blurams-mcp
```

Konfiguration ausschließlich über Umgebungsvariablen. Eine .env-Datei wird
**nicht automatisch geladen**. Die Beispiel-Datei enthält keine echten Secrets.

```sh
export BLURAMS_EMAIL='dein-konto@example.com'
read -rs -p 'Blurams-Passwort: ' BLURAMS_PASSWORD
export BLURAMS_PASSWORD
export MCP_TRANSPORT=stdio
./bin/blurams-mcp
```

Für einen MCP-Client das Binary als stdio-Server eintragen; die Variablen müssen
im gestarteten Prozess verfügbar sein. Erst list_cameras aufrufen. Nach
erfolgreicher Erkennung BLURAMS_CAMERA_ID festlegen und genau einen Schritt
testen. Zugangsdaten niemals in Chat, Git oder Issues senden.

## HTTP und Deployment

```sh
export MCP_TRANSPORT=http
export HTTP_ADDR=:8000
export MCP_BEARER_TOKEN="$(openssl rand -hex 32)"
./bin/blurams-mcp
```

Endpunkt: /mcp. Jeder MCP-HTTP-Aufruf benötigt Authorization: Bearer TOKEN.
Healthcheck: GET /health (ohne Zugangsdaten). Der Start verweigert HTTP mit
einem Token unter 32 Zeichen. Der Token hat keine eigene Ablaufzeit; durch
Änderung der Konfiguration und Neustart rotieren.

Dockerfile enthält einen mehrstufigen Go-Build und ein unprivilegiertes
Distroless-Laufzeitimage. render.yaml ist eine Deployment-Vorlage; es wurde
noch kein Dienst veröffentlicht. Nur hinter HTTPS betreiben. Secrets im
Secret Store des Hosters setzen. Zunächst eine Instanz verwenden; parallele
Bewegungsaufrufe werden derzeit nicht serialisiert.

## Sicherheit

- Blurams-Passwort und Sitzungstoken werden nur im Arbeitsspeicher gehalten.
- Kameraausgaben enthalten weder Passwort noch Token noch Product Key.
- Die vom Hersteller verlangte MD5-Passworttransformation dient nur der
  Protokollkompatibilität; sie ersetzt keine sichere Passwortspeicherung.
- Bewegungsgrenzen werden vor dem Bewegungsbefehl geprüft.
- Fehlende Erfolgsbestätigung wird als Fehler behandelt.
- HTTP-Authentifizierung verwendet die Bearer-Middleware des offiziellen SDK.
- Keine automatische Wiederholung von Bewegungen nach Netzwerkfehlern:
  Ein Fehler kann auftreten, nachdem die Kamera den Befehl erhalten hat.
- Undokumentierte Herstellerendpunkte können sich jederzeit ändern.

## Reproduzierbare Prüfung

```sh
go mod verify
go vet ./...
go test -race -coverprofile=coverage.out ./...
python3 scripts/mutation_test.py
go build -trimpath -o bin/blurams-mcp ./cmd/blurams-mcp
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Die Mutationstests injizieren gezielte Fehler in Signatur, Bewegungsgrenzen,
Kameraauswahl und Bearer-Prüfung und verlangen deren Erkennung. Sie sind ein
begrenzter, projektspezifischer Katalog, kein vollständiger Mutationstest des SDK.

Die GitHub-Actions-Prüfung führt Tests, Race Detector, Mutationstests,
Abhängigkeitsprüfung und Build aus. Die Ergebnisse des früheren Python-
Prototyps gelten nicht für diese Go-Implementierung.

## Struktur

- cmd/blurams-mcp: Start, HTTP, SIGTERM-Shutdown
- internal/config: Umgebungsvariablen und Startvalidierung
- internal/blurams: Cloud-Protokoll, Signaturen, PTZ und WebSocket-Relay
- internal/mcpserver: offizielle MCP-Schnittstelle und Zugangskontrolle
- scripts: gezielte Mutationstests

## Lizenz

MIT; siehe LICENSE. Abhängigkeiten behalten ihre jeweiligen Lizenzen.
