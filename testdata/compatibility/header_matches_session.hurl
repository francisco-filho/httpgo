GET http://example.invalid/capture
HTTP 200
[Asserts]
header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
GET http://example.invalid/health
HTTP 200
