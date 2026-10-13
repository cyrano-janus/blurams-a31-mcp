# Roadmap

Stand: 13. Oktober 2026. Diese Roadmap beschreibt geplante Arbeit; sie ist keine
Bestätigung einer funktionierenden realen Kameraanbindung oder eines Deployments.

## Ausgangsstand

- Go-Server mit offiziellem MCP-Go-SDK v1.8.0 implementiert.
- Werkzeuge: list_cameras, camera_status und move_camera.
- stdio und stateless Streamable HTTP; HTTP derzeit mit statischem Bearer-Token.
- Automatisierte Build-, Race-, Protokoll-, Mutation- und Sicherheitsprüfungen
  erfolgreich; Prüfgrenzen siehe [VERIFICATION.md](VERIFICATION.md).
- Reale Blurams-Anmeldung und A31-Steuerung noch nicht bestätigt.
- OAuth, Livestream, Bildanalyse und wiederkehrende Steuerung noch nicht eingerichtet.

## Gemeinsame Dienste mit pcloud-mcp

Das [pCloud-MCP-Projekt](https://github.com/cyrano-janus/pcloud-mcp) dient als
Referenz für die bereits implementierten Anmelde- und Sicherheitsmuster.
Die Übernahme erfordert Anpassung und eigene Tests für Blurams.

| Dienst | Geplante Verwendung | Trennung |
| --- | --- | --- |
| Render | Go-Hosting und HTTPS | Eigener Blurams-Dienst im bestehenden Account; pCloud-Dienst nicht verändern |
| Descope | MCP-OAuth, Consent und signierte Zugangstokens | Eigene MCP-Ressource und Audience für den Blurams-Endpunkt |
| GitHub | Quellcode und Deployment-Anbindung | Eigenes Repository blurams-a31-mcp |
| GitHub Actions | Build, Tests und Security-Prüfungen | Bestehende Blurams-CI erweitern |
| ChatGPT | Angemeldete Status- und Steuerungsaufrufe | Eigene MCP-Verbindung und explizite Berechtigungen |
| pCloud | Optional später Bilder und Aufnahmen speichern | Separater Dateizugang; erst nach Implementierung der Aufnahmefunktion |

Bestehende Accounts können genutzt werden. Tarifgrenzen, regionale Verfügbarkeit
und zusätzliche Kosten sind vor Einrichtung zu prüfen; ein kostenloser Betrieb
aller Erweiterungen wird nicht zugesagt.

## 1. Echte Blurams-Verbindung prüfen — offen, nächster Schritt

- Eine geschützte Testumgebung für den bestehenden Adapter einrichten; Render
  kann dafür als separater Dienst verwendet werden.
- Zugangsdaten ausschließlich über einen Secret Store bereitstellen.
- Zunächst nur list_cameras und camera_status mit der eingeschalteten A31 testen.
- Die erkannte Kamera-ID fest konfigurieren; danach einen einzelnen
  Bewegungsschritt mit Beobachtung der tatsächlichen Kamera testen.
- Scheitert der kompatible Ease-Life-Anmeldeweg, den tatsächlichen Blurams-Zugang
  untersuchen und einen passenden Adapter implementieren. Keine API-Kompatibilität
  aus der funktionierenden Smartphone-App ableiten.

**Abnahme:** Konto und Kamera werden korrekt erkannt; die fest freigegebene A31
führt einen einzelnen Befehl aus. Erfolgsbestätigung, Fehler und unklare
Befehlsergebnisse werden korrekt behandelt; keine Secrets in Ausgaben oder Logs.

## 2. Sicherheitsmuster aus pcloud-mcp übernehmen — offen

- Besitzerbindung, Netzwerkgrenzen, Secret-Verarbeitung, bereinigte Fehler,
  Rate Limits und Parallelitätsgrenzen auf Blurams anpassen.
- Statuszugriff und Bewegung getrennt freigeben; Bewegungen standardmäßig sperren,
  bis der Betreiber sie ausdrücklich aktiviert.
- Bewegungskommandos serialisieren und Grenzen serverseitig erzwingen.
- Das Muster verschlüsselter Zugangspakete prüfen und an Blurams anpassen;
  keine pCloud-Tokens oder pCloud-spezifischen OAuth-Helfer übernehmen.

**Abnahme:** Fremde Kamera-IDs und nicht freigegebene Bewegungen werden abgewiesen.
Befehle werden nach unklarem Netzwerkfehler nicht automatisch wiederholt.
Kritische Prüfungen sind durch Tests und gezielte Mutationstests abgesichert.

## 3. Descope-OAuth für den Blurams-MCP ergänzen — offen

- Einen externen Authorization Server verwenden, keinen eigenen Token-Aussteller bauen.
- Für Blurams eine eigene Descope-MCP-Ressource mit exakter Endpoint-Audience
  einrichten; bestehende pCloud-Ressource und deren Konfiguration beibehalten.
- Authorization Code mit PKCE S256 und passende MCP-Discovery einrichten.
- Scopes camera:read und camera:control getrennt definieren.
- Signatur, Issuer, Audience, festes Besitzer-Subject, Ablaufzeit und Scopes prüfen.
- Die bewährte pCloud-Verifier-Architektur wiederverwenden; Claim-Vertrag und
  Bibliotheksversionen für Blurams separat prüfen.
- Reale Token-Erneuerung, Ablauf, Widerruf und Schlüsselrotation testen.
  Bei lokaler JWT-Prüfung die verbleibende Gültigkeit bis zum Ablauf dokumentieren.

**Abnahme:** Ohne gültiges Token kein Kamerazugriff; fremde Benutzer, falsche
Audience, ungültige Signatur und fehlende Scopes werden abgewiesen.
Descope-Anmeldung und Blurams-Provider-Zugang bleiben unabhängig.

## 4. Render-Betrieb und ChatGPT-Abnahme — offen

- Separaten Blurams-Dienst mit HTTPS, geschützten Secrets und Healthcheck einrichten.
- Deployment reproduzierbar machen und erst nach erfolgreichen CI-Prüfungen freigeben.
- Geschützten MCP-Endpunkt, OAuth-Metadaten und Neustartverhalten prüfen.
- ChatGPT mit der eigenen Blurams-Ressource verbinden.
- Zunächst Statuszugriff, anschließend ausdrücklich freigegebene Bewegung live testen.

**Abnahme:** Anmeldung, Statusabfrage und kontrollierter Bewegungsschritt
funktionieren über ChatGPT. Unangemeldete Aufrufe bleiben gesperrt.
Keine Produktionsfreigabe allein aus Unit-Tests ableiten.

## 5. Regelmäßige Steuerung — offen, nach erfolgreicher Einzelabnahme

- Einen geeigneten Scheduler und dessen tatsächliche MCP-/OAuth-Unterstützung prüfen.
- Feste Kamera-ID, Zeitplan, Richtung und Schrittzahl konfigurieren.
- Betreiberaktivierung, Abschaltung, begrenzte Frequenz und nachvollziehbare
  Ergebnisse vorsehen; keine unbegrenzten Bewegungsfolgen.
- Verhalten bei Offline-Kamera, Token-Ablauf und unklarer Bestätigung testen.

**Abnahme:** Nur der freigegebene Ablauf läuft; Abschaltung und Fehlerbehandlung
sind live geprüft. Ein dauerhaft laufender KI-Agent wird nicht vorausgesetzt.

## 6. Optional: Aufnahme und pCloud-Ablage — offen

- Erst eine verfügbare, zulässige Blurams-Schnittstelle für Bilder oder Aufnahmen
  verifizieren und implementieren.
- Anschließend begrenzte pCloud-Ablage in einem eigenen freigegebenen Ordner prüfen.
- Speicherung ausdrücklich aktivieren, Zugriff und Aufbewahrung festlegen.
- Keine automatischen Lösch- oder Überschreiboperationen einführen.

**Abnahme:** Ein reales Bild oder eine Aufnahme wird kontrolliert übernommen,
mit eigenen Prüfungen für Inhalt, Dateigröße, Rechte und erfolgreiche Speicherung.

## Nachweise

Für jeden abgeschlossenen Schritt README und Prüfbericht aktualisieren.
Implementierung, simulierte Tests, echte Provider-Abnahme und Betriebsfreigabe
getrennt ausweisen. Es sind noch keine Termine für die offenen Schritte festgelegt.
