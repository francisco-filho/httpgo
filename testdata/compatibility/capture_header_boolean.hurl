GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Test"
POST http://example.invalid/health
Content-Type: application/json
{"value":{{x}}}
HTTP 200
