GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Test"
y: jsonpath "$.id"
POST http://example.invalid/health
Content-Type: application/json
{"header":{{x}},"id":{{y}}}
HTTP 200
