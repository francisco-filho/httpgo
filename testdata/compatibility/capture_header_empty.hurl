GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Test"
POST http://example.invalid/health
Content-Type: application/json
X-Probe: pre{{x}}post
{"value":{{x}},"text":"pre{{x}}post"}
HTTP 200
