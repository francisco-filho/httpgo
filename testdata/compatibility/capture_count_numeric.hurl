GET http://example.invalid/capture
HTTP 200
[Captures]
baseline: jsonpath "$" count
POST http://example.invalid/health
X-Probe: {{baseline}}
Content-Type: application/json
{"count":{{baseline}},"text":"{{baseline}}"}
HTTP 200
