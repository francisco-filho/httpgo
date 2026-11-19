GET {{base_url}}/first
GET {{base_url}}/late
HTTP 200
[Asserts]
body != "x"
