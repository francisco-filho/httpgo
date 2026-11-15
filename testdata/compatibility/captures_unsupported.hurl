GET {{base}}/health
HTTP 200
GET {{base}}/auth/login
HTTP 200
[Captures]
tokens: jsonpath "$.items[*]"
GET {{base}}/health
