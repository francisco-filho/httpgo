GET {{base}}/unavailable
HTTP 200
[Captures]
token: jsonpath "$.token"
GET {{base}}/health
