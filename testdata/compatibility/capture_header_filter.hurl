GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Test" regex "(.*)"
GET http://example.invalid/health
X-Probe: {{x}}
HTTP 200
