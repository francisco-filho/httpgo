GET http://example.invalid/capture
HTTP 201
[Captures]
x: regex "no-group"
GET http://example.invalid/health
X-Probe: pre{{x}}post
HTTP 200
