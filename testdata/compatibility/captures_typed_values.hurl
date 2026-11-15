GET {{base}}/capture
HTTP 200
[Captures]
flag: jsonpath "$.flag"
nothing: jsonpath "$.nothing"
empty: jsonpath "$.empty"
items: jsonpath "$.items"
object: jsonpath "$.object"
POST {{base}}/{{flag}}/{{nothing}}
Content-Type: application/json
X-Probe: {{flag}}-{{nothing}}
{"flag":{{flag}},"nothing":{{nothing}},"empty":{{empty}},"items":{{items}},"object":{{object}},"text":"{{flag}}-{{nothing}}"}
HTTP 200
