GET {{base}}/capture
[Captures]
token: jsonpath "$[0].token"
GET {{base}}/health
X-Probe: {{token}}
HTTP 200
