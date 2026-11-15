GET {{base}}/capture
HTTP 200
[Captures]
value: jsonpath "$"
GET {{base}}/health
X-Probe: {{value}}
HTTP 200
