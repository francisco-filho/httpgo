POST {{base_url}}/session/login
HTTP 200
[Captures]
old_token: jsonpath "$.accessToken"

POST {{base_url}}/session/renew
HTTP 200
[Asserts]
jsonpath "$.accessToken" != "{{old_token}}"
[Captures]
new_token: jsonpath "$.accessToken"

GET {{base_url}}/next
Authorization: Bearer {{new_token}}
HTTP 200
