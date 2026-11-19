GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "Absent"
GET http://example.invalid/health
X-Probe: {{x}}
HTTP 200
