GET {{base}}/auth/login
HTTP 200
[Captures]
token: jsonpath "$.missing"
GET {{base}}/health
X-Probe: {{token}}
