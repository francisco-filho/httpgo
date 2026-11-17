GET {{base_url}}/first
GET {{base_url}}/late
HTTP *
[Asserts]
jsonpath "$[**]" exists
