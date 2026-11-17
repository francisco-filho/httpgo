GET {{base_url}}/first
HTTP 200
GET {{base_url}}/late
HTTP 200
[Asserts]
body matches "{{pattern}}"
