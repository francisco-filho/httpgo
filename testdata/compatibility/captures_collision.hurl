GET {{base}}/auth/login
HTTP 200
[Captures]
token: jsonpath "$.accessToken"
GET {{base}}/health
X-Probe: {{token}}
HTTP 200
