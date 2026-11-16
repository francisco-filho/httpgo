GET {{base_url}}/first
GET {{base_url}}/late
HTTP *
[Asserts]
header "Set-Cookie" matches "private" "extra"
