GET http://example.invalid/capture
HTTP 200
[Asserts]
header "Absent" exists
[Captures]
x: regex "(absent)"
GET http://example.invalid/health
X-Probe: pre{{x}}post
HTTP 200
