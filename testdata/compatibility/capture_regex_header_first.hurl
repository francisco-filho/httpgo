GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Test" regex "(first,whole)"
GET http://example.invalid/health
X-Probe: pre{{x}}post
HTTP 200
