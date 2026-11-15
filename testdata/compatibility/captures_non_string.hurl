GET {{base}}/capture
HTTP 200
[Captures]
token: jsonpath "$.token"
GET {{base}}/health
X-Probe: {{token}}
