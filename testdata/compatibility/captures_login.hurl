POST {{base}}/auth/login
Content-Type: application/json
{"username":"demo","password":"senha-ficticia"}
HTTP 200
[Captures]
accessToken: jsonpath "$.accessToken"
empty: jsonpath "$.items[0]"
arrayToken: jsonpath "$.items[1]"

POST {{base}}/protected/{{accessToken}}
Authorization: Bearer {{accessToken}}
X-Probe: {{arrayToken}}|{{empty}}
Content-Type: application/json
{"full":"{{accessToken}}","text":"pre-{{accessToken}}"}
HTTP 200
