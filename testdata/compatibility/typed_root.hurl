POST {{base}}/health
Content-Type: application/json
{{value}}
HTTP 200
GET {{base}}/missing
HTTP 404
