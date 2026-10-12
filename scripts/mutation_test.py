"""Inject a finite catalog of security/protocol defects into isolated copies."""
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
MUTATIONS = [
    ("internal/blurams/critical.go", '"Arcsoft_" + password', '"Wrong_" + password'),
    ("internal/blurams/critical.go", 'strings.Join([]string{accessKey, timestamp, path, nonce}, "|")', 'strings.Join([]string{accessKey, timestamp, path, nonce}, ":")'),
    ("internal/blurams/critical.go", '[]byte(secret)', '[]byte("wrong")'),
    ("internal/blurams/critical.go", 'sha256.Sum256([]byte(body))', 'sha256.Sum256([]byte("wrong"))'),
    ("internal/blurams/critical.go", 'bits&0x10 != 0', 'bits&0x11 != 0'),
    ("internal/blurams/critical.go", 'bits&0x20 != 0', 'bits&0x21 != 0'),
    ("internal/blurams/critical.go", 'strings.ToLower(direction)', 'direction'),
    ("internal/blurams/critical.go", 'if !ok {', 'if ok {'),
    ("internal/blurams/critical.go", 'steps < 1', 'steps < 0'),
    ("internal/blurams/critical.go", 'steps > 5', 'steps > 6'),
    ("internal/mcpserver/server.go", 'len(expected) < 32', 'false'),
    ("internal/mcpserver/server.go", 'subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1', 'subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1'),
    ("internal/mcpserver/server.go", 'requested != s.cameraID', 'requested == s.cameraID'),
    ("internal/mcpserver/server.go", 'strings.ToLower(input.Direction)', 'strings.TrimSpace(input.Direction)'),
    ("internal/mcpserver/server.go", 'steps < 1', 'steps < 0'),
    ("internal/mcpserver/server.go", 'steps > 5', 'steps > 6'),
    ("internal/mcpserver/server.go", 'if !camera.SupportsPan {', 'if camera.SupportsPan {'),
    ("internal/blurams/relay.go", 'content.Response == nil || *content.Response != 0', 'content.Response == nil || *content.Response == 0'),
]

def run(directory):
    return subprocess.run(['go', 'test', '-count=1', './internal/...'], cwd=directory,
                          text=True, capture_output=True, timeout=90)

baseline = run(ROOT)
if baseline.returncode:
    raise SystemExit(baseline.stdout + baseline.stderr)
results = []
for index, (filename, before, after) in enumerate(MUTATIONS, 1):
    with tempfile.TemporaryDirectory(prefix='blurams-mutation-') as temporary:
        isolated = Path(temporary)
        for name in ('go.mod', 'go.sum'):
            shutil.copy2(ROOT / name, isolated / name)
        shutil.copytree(ROOT / 'internal', isolated / 'internal')
        path = isolated / filename
        source = path.read_text()
        if source.count(before) != 1:
            raise SystemExit(f'Mutation {index} no longer matches exactly once: {before}')
        path.write_text(source.replace(before, after))
        result = run(isolated)
        if '[build failed]' in result.stdout or 'undefined:' in result.stderr:
            raise SystemExit(f'Mutation {index} did not compile: {result.stdout}{result.stderr}')
        killed = result.returncode != 0
        results.append({'id': index, 'file': filename, 'detected': killed})
        print(f'{index}/{len(MUTATIONS)}: {"detected" if killed else "SURVIVED"}', flush=True)
(ROOT / 'mutation-results.json').write_text(json.dumps(results, indent=2) + '\n')
if not all(item['detected'] for item in results):
    raise SystemExit('Mutation test failed: surviving defects')
print(f'All {len(results)} targeted mutations detected.')
