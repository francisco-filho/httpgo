GET http://example.invalid/health
HTTP 200
GET http://example.invalid/capture
HTTP 200
[Captures]
x: regex "(?=private)(private)"
GET http://example.invalid/health
X-Probe: pre{{x}}post
HTTP 200
