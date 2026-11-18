# Guia de captures

**Capture extrai; assertion valida.** `[Captures]` dá nomes a valores recebidos
para reutilização; não prova por si só que esses valores atendem ao negócio.
As [assertions](assertions.md) da mesma resposta podem usar o lote provisório com
`--render-asserts`. O fluxo e a confirmação estão em [ordem e escopo](#ordem-escopo-e-renovação).
A referência normativa é o [subconjunto suportado](subset.md#capturas-tipadas).

## Pré-requisitos e primeira captura

Tenha `httpgo` no `PATH` ([instalação](../README.md#instalação)). Os comandos usam
shell POSIX; no PowerShell consulte `$LASTEXITCODE` em vez de `echo $?`.
Use uma pasta de trabalho separada do clone e somente dados fictícios.

Para este primeiro exemplo, inicie o [dummy](dev-server.md) na raiz do clone com
`go run ./tools/dummy/main.go`, mantendo-o aberto. `POST /products` responde
**201** com `{"id":1}`; `GET /products/1` responde **200** com
`{"id":1,"name":"Pizza","price":4500}`. São fixtures fixas, sem persistência.
Salve `encadeamento.httpgo`.

```hurl
POST {{base_url}}/products
Content-Type: application/json
{"name":"Pizza","price":4500}
HTTP 201
[Captures]
product_id: jsonpath "$.id"
[Asserts]
jsonpath "$.id" == 1

GET {{base_url}}/products/{{product_id}}
HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
```

```sh
httpgo test encadeamento.httpgo --var base_url=http://localhost:8080
```

Resultado **0**, dois envios aprovados, segundo path `/products/1`. O tipo de
`product_id` é número, não string. Trocar o esperado 1 por 2 falha com **1**,
descarta a captura e impede o GET. Se o POST responder `{}`, a captura ausente
falha com **2**, antes de qualquer assertion, sem GET. A CLI não imprime o valor
capturado nem o body. Encerre o dummy com Ctrl+C ao terminar.

## Catálogo de capturas

Uma seção `[Captures]` não pode ser órfã, duplicada na entrada ou vazia. Ela
pertence à resposta anterior, depois da expectativa quando presente; também
pode existir **sem `HTTP`** (nesse caso não há validação de status). Cada linha
é uma declaração; comentários só em linha própria, LF/CRLF são aceitos.
A próxima request-line encerra a seção; `[Asserts]` pode vir antes ou depois.

Nomes são exatos, com segmentos `[A-Za-z_][A-Za-z0-9_]*` separados por pontos.
`product.id` é um nome, não acesso a uma propriedade de objeto capturado.

| Forma completa em `[Captures]` | Valor publicado | Exemplo e resultado |
|---|---|---|
| `nome: jsonpath "consulta"` | Valor JSON tipado selecionado | `product_id: jsonpath "$.id"` com `{"id":1}` publica número `1` |
| `nome: jsonpath "consulta" count` | Número JSON inteiro não negativo | `orders_before: jsonpath "$.items" count` com `{"items":[1,null,3]}` publica `3` |
| `nome: header "Nome"` | Primeira ocorrência, string literal | `catalog_etag: header "ETag"` com `ETag: "v1"` publica a string contendo as aspas de `"v1"` |
| `nome: header "Nome" regex "padrão"` | Primeiro grupo da primeira correspondência, string | `refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"` seleciona `ficticio` de `refresh=ficticio; Path=/` |
| `nome: regex "padrão"` | Primeiro grupo da primeira correspondência no body textual, string | `otp_code: regex "verificação é: ([0-9]{6})"` seleciona `123456` do texto abaixo |

Não há outros filtros/transformações, pipelines, aritmética, escolha de grupo,
query `variable` ou consultas inteiras dinâmicas. Construção reconhecida não
suportada e sintaxe inválida são erros distintos, localizados, código **2**.
Erro estático em qualquer entrada impede **todo** HTTP do arquivo.

## JSONPath, tipos e seleção

Uma resposta JSON precisa conter **exatamente um valor JSON válido**. Captura
aceita string, número, booleano, `null`, array ou objeto, inclusive raiz `$`.
String numérica permanece string; números e números aninhados preservam precisão
e grafia, inclusive `9007199254740993` e `1.2300e+99`, sem conversão por float64.
`null`, `""`, `[]` e `{}` são valores presentes válidos. Campo ausente/índice
fora do array, JSON inválido/adicional e percurso incompatível são captura **2**.

### Gramática comum às capturas e assertions

| Passo | Exemplo | Contrato |
|---|---|---|
| Raiz | `$` | Seleciona o valor JSON inteiro |
| Propriedade por ponto | `$.product.id` | Propriedades simples ASCII com letras, dígitos e `_` |
| Índice decimal não negativo | `$.items[0].id` | Fora do array resulta em ausência |
| Wildcard de array | `$.categories[*].products[*].id` | Expande arrays em ordem, incluindo repetições |
| Filtro por igualdade | `$.items[?(@.slug=='pizza')].id` | Compara atributo do próprio item com literal escalar, sem coerção |

Passos são combináveis. O filtro aceita caminho relativo por propriedades/índices
e literal string (aspas simples/duplas), número JSON exato, booleano ou `null`.
Atributo ausente não corresponde nem a `null`. Não há wildcard de objeto `.*`,
propriedade entre colchetes, índice negativo, slices, união, descida recursiva,
funções/scripts, regex, operadores lógicos ou operadores de filtro além de `==`.

Há duas camadas de escapes: string externa JSON do cenário e string interna do
filtro. A consulta `"$[?(@.name=='it\\'s pizza')].id"` busca `it's pizza`;
`"$[?(@.name==\"a\\\"b\")].id"` busca `a"b`. A string interna aceita escapes
JSON e `\'` em aspas simples; cada barra precisa de escape novamente na externa.
Veja [mais exemplos de filtros](subset.md#seleção-estática-por-igualdade).

### Cardinalidade e percurso estrito

Ao final da consulta, **zero resultados é ausência; um é o próprio valor;
vários formam array ordenado**, sem achatamento nem deduplicação. Um único array
conserva conteúdo e tipo, inclusive vazio; vários arrays são elementos do array
final. Propriedade ausente/índice fora do limite elimina só aquele ramo;
`null` selecionado permanece. Propriedade em não objeto ou índice/wildcard/filtro
em não array gera **2**, mesmo quando outros ramos seriam válidos.

Por exemplo, com `{"issues":[{"code":"INVALID"},{"code":"REQUIRED"}]}`,
`$.issues[*].code` publica `["INVALID","REQUIRED"]`. Com só um item publica
string; sem resultados é ausência, não `[]`. Selecionar diretamente
`$.issues` quando vale `[]` é válido e publica array vazio.

Para executar a projeção, requer **outra API** (não o dummy): `GET /validation`
com 200 e o JSON acima; `POST /echo` aceita esse array e retorna **204**. Salve
`projecao.httpgo` ou execute a fixture na raiz do clone:

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.issues[*].code" contains "REQUIRED"
jsonpath "$.issues[*].code" not contains "OTHER_TENANT"
[Captures]
codes: jsonpath "$.issues[*].code"
POST {{base_url}}/echo
{{codes}}
HTTP 204
```

```sh
httpgo test testdata/scenarios/scenario_projection.httpgo --var base_url=http://localhost:8081
```

Resultado **0**, dois envios; sem REQUIRED falha **1**, sem POST. Sem nenhum
`code`, a captura é ausente e falha **2** antes das assertions. Nada aplica uma
assertion individualmente a cada ramo: ela recebe o valor final reduzido.

### Filtro estático

API de testes em 8081: `GET /products` responde 200 com
`[{"slug":"pizza","id":1,"active":true}]`; `GET /products/1` responde 200.
Salve `filtro.httpgo` ou use a fixture:

```hurl
GET {{base_url}}/products
HTTP 200
[Captures]
pizza_id: jsonpath "$[?(@.slug=='pizza')].id"
[Asserts]
jsonpath "$[?(@.slug=='pizza')].active" == true

GET {{base_url}}/products/{{pizza_id}}
HTTP 200
```

```sh
httpgo test testdata/scenarios/scenario_filter.httpgo --var base_url=http://localhost:8081
```

Resultado **0**, dois envios. Sem pizza: captura **2**, sem GET final. Múltiplos
IDs formam array; o uso desse array em URL é renderização **2**, nunca seleção
implícita do primeiro ID.

### Operandos dinâmicos: opt-in

`--render-asserts` é um **opt-in HTTPGo**, necessário também para operandos de
igualdade em consultas de **captura**. Sem ele, placeholder em consulta é erro
estático **2**, inclusive dentro de string e com variável definida.

API de testes em 8081: `GET /first` responde 200 e `{"number":42}`;
`GET /orders`, 200 e
`{"items":[{"number":42,"status":"PLACED","id":"selected"}]}`;
`GET /selected`, 200. Salve `filtro-dinamico.httpgo` ou use a fixture:

```hurl
GET {{base_url}}/first
HTTP 200
[Captures]
order_number: jsonpath "$.number"

GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number=={{order_number}})].status" == "PLACED"
[Captures]
selected_id: jsonpath "$.items[?(@.number=={{order_number}})].id"

GET {{base_url}}/{{selected_id}}
HTTP 200
```

```sh
httpgo test testdata/scenarios/dynamic_filter.httpgo --render-asserts --var base_url=http://localhost:8081
```

Resultado **0**, três envios. Só o operando pode variar: placeholder completo
conserva tipo **escalar** (string/número/booleano/`null`); dentro de literal,
como `'Order {{suffix}}'`, compõe texto escalar. Arrays/objetos são **2**, até
como operando completo. Valores são vinculados ao plano estático; não são
concatenados/reanalisados como JSONPath, nem expandidos recursivamente.

Propriedades, índices, operadores, caminhos relativos e consulta inteira não
interpolam. Todos os operandos de uma consulta alcançada são resolvidos antes
de selecionar/decodificar, mesmo sem itens. Variável ausente/tipo inválido é **2**.
Capturas só consultam valores **iniciais ou confirmados anteriores**, nunca
outra captura do lote atual. Assertions veem o lote provisório válido; o esperado
da assertion é preparado antes dos seus operandos de consulta.

## Capturar contagem e reutilizar em assertion

`count` conta elementos do **array final selecionado**, incluindo `null`.
Array vazio publica número `0`. Ausência não é baseline zero; `null`, string,
número, booleano ou objeto não são arrays: erro de captura **2**. JSON inválido,
valor adicional ou percurso não avaliável também são **2**. Com projeção,
um escalar único não é contável; vários valores contam como elementos, e
vários arrays não têm os elementos internos somados.

Requer API de testes em 8081: `GET /orders` retorna 200 com `{"items":[1,2,3]}`
antes/depois; `POST /orders/preview` aceita `{"productId":"p1"}` e retorna 200,
sem mudar quantidade. Salve `contagem.httpgo` ou execute a fixture:

```hurl
GET {{base_url}}/orders

HTTP 200
[Captures]
orders_before: jsonpath "$.items" count

POST {{base_url}}/orders/preview
Content-Type: application/json
{"productId":"p1"}

HTTP 200

GET {{base_url}}/orders

HTTP 200
[Asserts]
jsonpath "$.items" count == {{orders_before}}
```

```sh
httpgo test testdata/scenarios/scenario_count_unchanged.httpgo --render-asserts --var base_url=http://localhost:8081
```

Resultado **0** com três itens finais; quatro itens finais dão **1**, não erro
de captura. Trocar conteúdo sem mudar quantidade passa. A captura/reutilização
em requisição não exige flag; **o esperado dinâmico da assertion exige
`--render-asserts`**. Sem ela, o esperado bare acima é rejeitado antes de HTTP.
Não há parâmetros, soma/média, aritmética, contagem de strings/objetos/headers ou
pipelines. Quantidade permanece número JSON exato na reutilização.

## Header literal

O nome do header é token HTTP **estático**, entre aspas duplas, sem distinguir
caixa. Só a **primeira ocorrência** participa, inclusive vazia; não concatena
valores nem divide vírgulas. Ausência é captura **2**; presente vazio publica
string vazia. Aspas, números, JSON ou placeholders recebidos são texto literal,
sem coerção ou expansão recursiva. Não exige body JSON, mas toda resposta ainda
precisa ser consumida e fechada.

API de cache em 8081: `GET /catalog` retorna 200 e `ETag: "v1"`; com
`If-None-Match: "v1"`, retorna 304. Salve `cache.httpgo` ou use a fixture:

```hurl
GET {{base_url}}/catalog
HTTP 200
[Captures]
catalog_etag: header "ETag"

GET {{base_url}}/catalog
If-None-Match: {{catalog_etag}}
HTTP 304
```

```sh
httpgo test testdata/scenarios/scenario_capture_cache.httpgo --var base_url=http://localhost:8081
```

Resultado **0**, dois envios; sem ETag, **2**, sem segundo envio. Não há índice,
consulta de todas as ocorrências ou cookie jar. Capturar `Set-Cookie` não cria
`Cookie` nem remove atributos automaticamente.

## Regex de header e body

Padrões de **extração** são strings JSON **estáticas**, inclusive com
`--render-asserts`. Sintaxe de `regexp` Go: classes, grupos, alternativas,
repetições, âncoras e opções `(?i)`, `(?m)`, `(?s)`; busca por trecho, sem
normalização Unicode. `"\\d"` entrega `\d` à regex; `"\x41"` é escape JSON
inválido. Sem delimitadores `/.../`, lookaround ou backreferences. Regex
malformada/não suportada e escapes inválidos impedem todo HTTP com **2**.

A seleção é **o primeiro grupo da primeira correspondência**, sempre string.
Não seleciona a correspondência inteira, outro grupo ou match posterior.
Primeiro grupo participante vazio publica `""`; sem match, sem grupo ou com
primeiro grupo opcional não participante é captura **2**. Regex válida sem
nenhum grupo só falha na extração. Não há substituição, grupo numerado ou
padrão de extração dinâmico.

Body usa o texto **final completo**, UTF-8 estrito por padrão ou charset do
primeiro Content-Type. Charset desconhecido/conversão inválida é **2**; não
normaliza JSON, caixa, whitespace ou Unicode. ISO-8859-1 segue o índice web
(windows-1252). Header usa só a primeira ocorrência, sem decodificar body.

### Extrair código de texto

API em 8081: `GET /message/123` retorna 200 e
`Seu código de verificação é: 123456`; `POST /verify` aceita
`{"code":"123456"}` e retorna 200. Salve `codigo.httpgo` ou use a fixture:

```hurl
GET {{mailbox_url}}/message/123
HTTP 200
[Captures]
otp_code: regex "verificação é: ([0-9]{6})"

POST {{base_url}}/verify
Content-Type: application/json
{"code":"{{otp_code}}"}
HTTP 200
```

```sh
httpgo test testdata/scenarios/scenario_capture_otp.httpgo --var mailbox_url=http://localhost:8081 --var base_url=http://localhost:8081 --secret otp_code
```

Resultado **0**, dois envios; sem correspondência, **2**, sem POST. O código
fictício é protegido pela marca explícita, não por detecção automática do body.
Para validar também o grupo, consulte a query de extração regex no
[catálogo de assertions](assertions.md#catálogo-de-extração-regex-do-body).

### Extrair cookie explicitamente

API em 8081: `GET /session` retorna 200 e
`Set-Cookie: refresh=ficticio; Path=/`; `GET /next` aceita
`Cookie: refresh=ficticio` e retorna 200. Salve `cookie.httpgo` ou use a fixture:

```hurl
GET {{base_url}}/session
HTTP 200
[Captures]
refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"

GET {{base_url}}/next
Cookie: refresh={{refresh_token}}
HTTP 200
```

```sh
httpgo test testdata/scenarios/scenario_capture_refresh.httpgo --var base_url=http://localhost:8081 --secret refresh_token
```

Resultado **0**; header ausente, sem match ou grupo ausente é **2**, sem GET
final. Envia somente o cookie extraído, não atributos de Set-Cookie. Headers
sensíveis protegem automaticamente também o grupo extraído; `--secret` torna
explícita a intenção.

## Reutilização, ordem e escopo

### Reutilizar preservando tipos

Em URL, valor de header ou dentro de string JSON, só escalares: string usa
conteúdo, número usa grafia JSON, booleano usa `true`/`false`, `null` usa texto
`null`. Arrays/objetos exigem posição de **valor JSON completo**, inclusive raiz,
como `{"value":{{captura}}}` ou `{{captura}}`. O tipo e a precisão são preservados.
Objetos capturados são opacos; não criam variáveis por propriedade.

Strings JSON são escapadas corretamente; conteúdo inserido não é um template
novo. Não há percent-encoding automático de URL; forneça encoding necessário.
URL final inválida, CR/LF/controles indevidos em header, variável ausente ou uso
textual de coleção é renderização **2**, sem envio da entrada afetada/posteriores.
Para body externo, somente `--render-files` habilita templates JSON, uma
[extensão HTTPGo separada](subset.md#extensão-httpgo-json-externo-com---render-files).

### Ordem, escopo e renovação

A ordem vigente, **independente da ordem das seções**, é:

1. Consumir e fechar a resposta completa; validar status quando declarado.
2. Extrair e validar **todas** as capturas provisórias usando só iniciais ou
   confirmados anteriores. Não há encadeamento entre capturas do próprio lote.
3. Executar assertions na ordem escrita. Todas veem o lote provisório válido,
   que prevalece sobre confirmados/iniciais; a flag de esperados dinâmicos
   continua necessária. Captura inválida retorna **2 antes de qualquer assertion**.
4. Com status/assertions aprovados e contexto ativo, **confirmar o lote inteiro**.
   Sem assertions, confirma depois de status aceito. Sem capturas, usa a sessão
   já confirmada. Sem HTTP não presume status validado; `HTTP *` aceita qualquer.

Captura pode renovar um **nome exatamente igual**, inclusive inicial e mudando
tipo. Por exemplo, duas respostas sucessivas `{"id":1}`/`{"id":"novo"}` com
`id: jsonpath "$.id"` renovam número para string após cada confirmação.
Para comparar antigo/novo, use nomes distintos. Duplicação no mesmo lote e
conflito ancestral/descendente (`item`/`item.id`), em qualquer direção, são **2**.
Não muda a regra de duplicação de variáveis CLI/YAML.

Divergência de status é barreira **1 anterior às capturas**. Assertion divergente
é **1** e descarta lote; captura/avaliação inválida, timeout ou cancelamento é
**2** e não confirma. Todos preservam valores anteriores/ausência dos nomes novos,
impedem próximo envio e **não desfazem efeitos da API**. Falha de saída/escrita
curta/fechamento prevalece com **2**. Resultados anteriores são preservados.

Com [polling opt-in](subset.md#polling-local-por-entrada), só divergência **1**
permite nova tentativa no orçamento; ela reenvia com os valores confirmados
**antes da entrada**, nunca com capturas reprovadas. Erro **2** encerra.
Sem esse opt-in, não há reenvio automático.

Cada arquivo/executação começa com contexto inicial imutável e sessão nova.
Capturas confirmadas não passam para outro arquivo, mesmo na mesma suíte.
A proteção acumulada da invocação pode atravessar arquivos; isso **não** torna
valores capturados resolvíveis em outro arquivo.

## Sigilo e resultados

`--secret nome` e `secrets` YAML marcam nomes, inclusive capturas futuras, **sem
fornecer valor**. Credenciais iniciais reais vêm do ambiente do processo, nunca
de flags: veja [configuração segura](configuration.md#referências-ao-ambiente-do-processo-e-segredos).
Authorization, Proxy-Authorization, Cookie e Set-Cookie são protegidos sem
distinguir caixa. Dados secretos em JSON/body exigem marcação explícita.

Proteção é **monotônica**: marcas e valores secretos conhecidos anteriores,
novos e derivados permanecem após descarte/renovação e troca de arquivo na
invocação. Um segredo descoberto é protegido antes de outra captura falhar;
constituintes de coleções/variantes conhecidas também são protegidos, sem virarem
variáveis ou alterar bytes enviados. Vazio não causa substituição global.
Captura secreta pendente oculta operandos de assertions sem confirmar o lote.

Diagnósticos têm arquivo/linha/coluna, consulta/transformação e causa segura;
não despejam body, coleção de headers, valores capturados, grupo regex ou
seleções compostas. Resultados/callbacks/JUnit não retêm planos/padrões brutos.
Segredos arbitrários não marcados e transformações/codificações desconhecidas
não têm detecção garantida. Stderr já emitido não pode ser redigido retroativamente.

**0**: validações aprovadas; **1**: divergência de assertion/status;
**2**: captura/entrada/configuração/renderização/avaliação/transporte/cancelamento/
escrita. Em suíte, 2 prevalece sobre 1. Stdout recebe resultados; stderr,
diagnósticos. Falha de captura não transforma resposta completa em erro de
transporte, nem apaga status já validado.

Referências: [JSONPath](subset.md#capturas-tipadas),
[count](subset.md#captura-de-contagem), [header](subset.md#capturas-literais-de-headers),
[regex](subset.md#extração-regex-de-body-e-header),
[transação](subset.md#renovação-transacional-de-capturas) e
[diferenças de compatibilidade](compatibility.md). Não execute toda
`testdata/scenarios` como suíte: inclui entradas intencionalmente inválidas.
