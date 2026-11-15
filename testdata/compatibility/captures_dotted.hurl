GET {{base}}/auth/login
HTTP 200
[Captures]
auth.token: jsonpath "$.accessToken"
GET {{base}}/health
X-Probe: {{auth.token}}
HTTP 200
