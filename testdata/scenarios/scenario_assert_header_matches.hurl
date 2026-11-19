GET {{base_url}}/session

HTTP 200
[Asserts]
header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
