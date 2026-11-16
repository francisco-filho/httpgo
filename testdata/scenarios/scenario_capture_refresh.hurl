GET {{base_url}}/session
HTTP 200
[Captures]
refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"

GET {{base_url}}/next
Cookie: refresh={{refresh_token}}
HTTP 200
