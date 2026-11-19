GET http://example.invalid/capture
HTTP 200
[Captures]
x: regex "código: (.+)"
GET http://example.invalid/health
X-Probe: pre{{x}}post
HTTP 200
