GET {{base_url}}/first
GET {{base_url}}/late
HTTP *
[Asserts]
header "X" not exists
