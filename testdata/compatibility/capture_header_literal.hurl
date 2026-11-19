GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Test"
POST http://example.invalid/health
Content-Type: application/json
X-Probe: {{x}}
{"value":{{x}},"text":"prefix{{x}}"}
HTTP 200
